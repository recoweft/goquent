package tests

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/query"
)

// boundaryProxy owns only its listener and accepted connection pairs. It decodes
// request framing, not TCP read boundaries, and never logs payloads. TLS and
// non-loopback fixtures are deliberately outside this finite test harness.
type boundaryProxy struct {
	listener                     net.Listener
	target, dialect              string
	armed                        atomic.Bool
	entered                      chan struct{}
	stop                         chan struct{}
	once                         sync.Once
	mu                           sync.Mutex
	conns                        []net.Conn
	auditTarget                  string
	targetExplain, targetExecute int
	wg                           sync.WaitGroup
}

func newBoundaryProxy(t *testing.T, dialect, dsn string) (*boundaryProxy, string) {
	t.Helper()
	p := &boundaryProxy{dialect: dialect, entered: make(chan struct{}), stop: make(chan struct{})}
	var cfg *mysql.Config
	var u *url.URL
	var err error
	if dialect == "mysql" {
		cfg, err = mysql.ParseDSN(dsn)
		if err == nil {
			p.target = cfg.Addr
		}
	} else {
		u, err = url.Parse(dsn)
		if err == nil {
			p.target = u.Host
		}
	}
	if err != nil {
		t.Fatal("proxy fixture DSN invalid")
	}
	p.listener, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal("proxy listener unavailable")
	}
	if cfg != nil {
		cfg.Addr = p.listener.Addr().String()
		dsn = cfg.FormatDSN()
	} else {
		u.Host = p.listener.Addr().String()
		dsn = u.String()
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			client, err := p.listener.Accept()
			if err != nil {
				return
			}
			server, err := net.DialTimeout("tcp", p.target, 2*time.Second)
			if err != nil {
				client.Close()
				continue
			}
			p.mu.Lock()
			select {
			case <-p.stop:
				client.Close()
				server.Close()
				p.mu.Unlock()
				return
			default:
			}
			p.conns = append(p.conns, client, server)
			p.mu.Unlock()
			p.wg.Add(1)
			go p.pair(client, server)
		}
	}()
	t.Cleanup(func() { p.close(t) })
	return p, dsn
}

func (p *boundaryProxy) pair(client, server net.Conn) {
	defer p.wg.Done()
	defer client.Close()
	defer server.Close()
	var hold atomic.Bool
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		defer client.Close()
		defer server.Close()
		buf := make([]byte, 32*1024)
		for {
			n, err := server.Read(buf)
			if n > 0 {
				if hold.Load() {
					p.once.Do(func() { close(p.entered) })
					<-p.stop
					return
				}
				if _, e := client.Write(buf[:n]); e != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	// PostgreSQL startup and cancellation packets have no type byte.
	startup := p.dialect == "postgres"
	for {
		headerLen := 4
		if p.dialect == "postgres" && !startup {
			headerLen = 5
		}
		header := make([]byte, headerLen)
		if _, err := io.ReadFull(client, header); err != nil {
			break
		}
		length := 0
		if p.dialect == "mysql" {
			length = int(header[0]) | int(header[1])<<8 | int(header[2])<<16
		} else {
			length = int(binary.BigEndian.Uint32(header[headerLen-4:])) - 4
		}
		if length < 0 || length > 1<<20 {
			break
		}
		body := make([]byte, length)
		if _, err := io.ReadFull(client, body); err != nil {
			break
		}
		statement := ""
		if p.dialect == "mysql" && len(body) > 0 && body[0] == 3 {
			statement = string(body[1:])
		}
		if p.dialect == "postgres" && !startup && (header[0] == 'Q' || header[0] == 'P') {
			statement = string(body)
		}

		p.mu.Lock()
		if p.auditTarget != "" && strings.Contains(statement, p.auditTarget) {
			if strings.HasPrefix(strings.TrimLeft(statement, "\x00"), "EXPLAIN ") {
				p.targetExplain++
			} else {
				p.targetExecute++
			}
		}
		p.mu.Unlock()
		// The version request occurs only after the native read-only Begin succeeded.
		// Hold its real server response, after forwarding the request in full.
		if p.armed.Load() && (strings.Contains(statement, "SELECT VERSION(), DATABASE()") || strings.Contains(statement, "SELECT pg_catalog.current_setting('server_version')")) {
			hold.Store(true)
		}
		if _, err := server.Write(append(header, body...)); err != nil {
			break
		}
		startup = false
	}
	client.Close()
	server.Close()
	// close() releases a held response before joining this worker.
	<-finished
}

func (p *boundaryProxy) close(t *testing.T) {
	t.Helper()
	p.mu.Lock()
	select {
	case <-p.stop:
	default:
		close(p.stop)
	}
	p.listener.Close()
	for _, conn := range p.conns {
		conn.Close()
	}
	p.mu.Unlock()
	done := make(chan struct{})
	go func() { p.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Error("proxy cleanup deadline exceeded")
	}
}

func boundaryNetwork(t *testing.T, owner *orm.DB, dialect, dsn, table, fault string) {
	proxy, proxiedDSN := newBoundaryProxy(t, dialect, dsn)
	db, err := orm.OpenWithDriver(dialect, proxiedDSN)
	if err != nil {
		t.Fatal("proxied native connection failed")
	}
	t.Cleanup(func() {
		if db.Close() != nil {
			t.Error("proxy pool cleanup failed")
		}
	})
	db.SQLDB().SetMaxOpenConns(1)
	boundaryControl(t, db, table)
	var session int64
	identitySQL := "SELECT CONNECTION_ID()"
	if dialect == "postgres" {
		identitySQL = "SELECT pg_backend_pid()"
	}
	identityCtx, identityCancel := context.WithTimeout(t.Context(), 3*time.Second)
	identityErr := db.SQLDB().QueryRowContext(identityCtx, identitySQL).Scan(&session)
	identityCancel()
	if identityErr != nil || session <= 0 {
		t.Fatal("fault session identity unavailable")
	}
	proxy.armed.Store(true)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	type result struct {
		p   *query.PerformancePlan
		err error
	}
	done := make(chan result, 1)
	go func() {
		p, err := db.Table(table).Select("id").Limit(1).ExplainPerformance(ctx, query.PerformanceOptions{})
		done <- result{p, err}
	}()
	select {
	case <-proxy.entered:
	case <-time.After(3 * time.Second):
		proxy.close(t)
		t.Fatal("fault synchronization unavailable")
	}
	want := query.ErrPerformanceCleanup
	switch fault {
	case "cancel":
		cancel()
		want = query.ErrPerformanceCanceled
	case "timeout":
		<-ctx.Done()
		want = query.ErrPerformanceTimeout
	}
	// Teardown after the explicit deadline/cancel also bounds drivers which await
	// a server cancellation reply. It is part of this fault scope, not a claim of
	// unlimited driver cancellation support under an indefinitely stalled peer.
	proxy.close(t)
	select {
	case r := <-done:
		boundaryRefusal(t, r.p, r.err, want)
	case <-time.After(3 * time.Second):
		t.Fatal("fault acquisition deadline exceeded")
	}
	if db.Close() != nil || db.SQLDB().Stats().InUse != 0 {
		t.Fatal("fault pool cleanup failed")
	}
	// Check the real server session disappeared, including its owned read-only
	// transaction. Poll a condition with a deadline, never a sleep-based race.
	cleanupCtx, cleanupCancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cleanupCancel()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		countSQL := "SELECT COUNT(*) FROM information_schema.PROCESSLIST WHERE ID=?"
		if dialect == "postgres" {
			countSQL = "SELECT COUNT(*) FROM pg_catalog.pg_stat_activity WHERE pid=$1"
		}
		var remaining int
		if owner.SQLDB().QueryRowContext(cleanupCtx, countSQL, session).Scan(&remaining) != nil {
			t.Fatal("server cleanup observation failed")
		}
		if remaining == 0 {
			break
		}
		select {
		case <-tick.C:
		case <-cleanupCtx.Done():
			t.Fatal("server transaction cleanup deadline exceeded")
		}
	}
	boundaryControl(t, owner, table)
}
