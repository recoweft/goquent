//go:build linux

package review

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// Each probe runs in a separate process with an explicit credential-free env.
// The supervisor owns a process group and a hard deadline independent of Load.
func TestSecurityProcessDriverBoundary(t *testing.T) {
	for _, mode := range []string{"explicit", "path", "cancel_child", "native_replace", "native_workspace"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			binary, err := os.Executable()
			if err != nil {
				t.Fatal("helper unavailable")
			}
			driver := filepath.Join(dir, "gopackagesdriver")
			if os.WriteFile(driver, []byte("#!/bin/sh\nGQ_ROLE=driver exec \"$GQ_HELPER\" -test.run=^TestSecurityProcessHelper$ -- \"$@\"\n"), 0700) != nil {
				t.Fatal("driver unavailable")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, "-test.run=^TestSecurityProcessHelper$", "-test.timeout=12s")
			cmd.Env = []string{"PATH=" + dir + ":" + filepath.Join(runtime.GOROOT(), "bin") + ":/usr/bin:/bin", "HOME=" + dir,
				"GQ_ROLE=load", "GQ_MODE=" + mode, "GQ_DIR=" + dir, "GQ_HELPER=" + binary,
				"GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOENV=off", "GOWORK=off", "GOFLAGS=-tags=fixture_probe"}
			if mode != "path" {
				cmd.Env = append(cmd.Env, "GOPACKAGESDRIVER="+driver)
			}
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			cmd.WaitDelay = time.Second
			cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
			if cmd.Start() != nil {
				t.Fatal("probe start failed")
			}
			defer syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			if cmd.Wait() != nil {
				t.Fatal("isolated driver probe failed")
			}
		})
	}
}

func securityWaitFile(t *testing.T, path string) []byte {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(path); err == nil {
			return b
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("fixture handshake timed out")
	return nil
}

func TestSecurityProcessHelper(t *testing.T) {
	role, dir := os.Getenv("GQ_ROLE"), os.Getenv("GQ_DIR")
	if role == "" {
		return
	}
	write := func(name string, b []byte) {
		if os.WriteFile(filepath.Join(dir, name), b, 0600) != nil {
			t.Fatal("fixture handshake failed")
		}
	}
	switch role {
	case "descendant":
		write("child-ready", []byte(strconv.Itoa(os.Getpid())))
		securityWaitFile(t, filepath.Join(dir, "release-child"))
		write("child-after-cancel", []byte("fixture"))
		os.Exit(0)
	case "driver":
		var req struct {
			Env        []string
			BuildFlags []string `json:"build_flags"`
			Tests      bool
			Overlay    map[string][]byte
		}
		if json.NewDecoder(os.Stdin).Decode(&req) != nil {
			os.Exit(2)
		}
		cwd, _ := os.Getwd()
		if cwd != dir || !req.Tests || !slices.Contains(req.BuildFlags, "-mod=readonly") ||
			!slices.Contains(req.Env, "GOFLAGS=-tags=fixture_probe") || !slices.Contains(req.Env, "GOENV=off") ||
			!slices.Contains(req.Env, "GOWORK=off") || string(req.Overlay[filepath.Join(dir, "input.go")]) != "package fixture\n// overlay\n" {
			os.Exit(3)
		}
		write("driver-observed", []byte("expected"))
		if os.Getenv("GQ_MODE") == "cancel_child" {
			cmd := exec.Command(os.Getenv("GQ_HELPER"), "-test.run=^TestSecurityProcessHelper$")
			cmd.Env = append(os.Environ(), "GQ_ROLE=descendant")
			// No inherited output pipes: cancellation return and descendant lifetime
			// are separate observations. Inherited-pipe hangs remain unverified here.
			if cmd.Start() != nil {
				os.Exit(4)
			}
			_ = cmd.Wait()
		}
		_, _ = io.WriteString(os.Stdout, `{"Compiler":"gc","Arch":"amd64","Roots":[],"Packages":[]}`)
		os.Exit(0)
	case "load":
		if mode := os.Getenv("GQ_MODE"); mode == "native_replace" || mode == "native_workspace" {
			securityNativeLoad(t, dir, mode)
			return
		}
		// Reap the deliberately orphaned fixture child ourselves after cancellation.
		// Linux PR_SET_CHILD_SUBREAPER, only in this isolated test helper.
		if _, _, e := syscall.Syscall6(syscall.SYS_PRCTL, 36, 1, 0, 0, 0, 0); e != 0 {
			t.Fatal("cleanup supervisor unavailable")
		}
		write("go.mod", []byte("module fixture\n\ngo 1.25.0\n"))
		write("input.go", []byte("package fixture\n"))
		path := filepath.Join(dir, "input.go")
		overlay := map[string][]byte{path: []byte("package fixture\n// overlay\n")}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan map[string]*typedFile, 1)
		go func() { done <- loadReviewTypesSnapshot(ctx, []string{path}, overlay) }()
		child := 0
		if os.Getenv("GQ_MODE") == "cancel_child" {
			pid := securityWaitFile(t, filepath.Join(dir, "child-ready"))
			child, _ = strconv.Atoi(string(pid))
			if child <= 0 {
				t.Fatal("child identity unavailable")
			}
			defer func() {
				if child > 0 {
					_ = syscall.Kill(child, syscall.SIGKILL)
					_, _ = syscall.Wait4(child, nil, 0, nil)
				}
			}()
			cancel()
		}
		select {
		case result := <-done:
			if result[path] == nil || !result[path].incomplete {
				t.Fatal("empty driver promoted to complete")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("load did not return")
		}
		securityWaitFile(t, filepath.Join(dir, "driver-observed"))
		if child > 0 {
			if syscall.Kill(child, 0) != nil {
				t.Fatal("descendant observation changed")
			}
			write("release-child", []byte("release after load returned"))
			securityWaitFile(t, filepath.Join(dir, "child-after-cancel"))
			var status syscall.WaitStatus
			if _, err := syscall.Wait4(child, &status, 0, nil); err != nil || !status.Exited() || status.ExitStatus() != 0 {
				t.Fatal("child cleanup failed")
			}
			child = 0
		}
	}
}

func securityNativeLoad(t *testing.T, dir, mode string) {
	root, dep := filepath.Join(dir, "analysis"), filepath.Join(dir, "dependency")
	for _, p := range []string{root, dep} {
		if os.Mkdir(p, 0700) != nil {
			t.Fatal("native fixture directory failed")
		}
	}
	files := map[string]string{
		filepath.Join(dep, "go.mod"):    "module example.test/dependency\n\ngo 1.25.0\n",
		filepath.Join(dep, "dep.go"):    "package dependency\ntype Value int\n",
		filepath.Join(root, "input.go"): "//go:build fixture_probe\n\npackage fixture\nimport d \"example.test/dependency\"\nvar Value d.Value\n",
		filepath.Join(dir, "goenv"):     "GOFLAGS=-tags=fixture_probe\n",
	}
	mod := "module fixture\n\ngo 1.25.0\nrequire example.test/dependency v0.0.0\n"
	if mode == "native_replace" {
		mod += "replace example.test/dependency => ../dependency\n"
	} else {
		files[filepath.Join(dir, "go.work")] = "go 1.25.0\nuse (\n./analysis\n./dependency\n)\n"
		t.Setenv("GOWORK", filepath.Join(dir, "go.work"))
	}
	files[filepath.Join(root, "go.mod")] = mod
	for p, contents := range files {
		if os.WriteFile(p, []byte(contents), 0600) != nil {
			t.Fatal("native fixture write failed")
		}
	}
	t.Setenv("GOPACKAGESDRIVER", "off")
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOENV", filepath.Join(dir, "goenv"))
	t.Setenv("GOCACHE", filepath.Join(dir, "cache"))
	path := filepath.Join(root, "input.go")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	result := loadReviewTypes(ctx, []string{path})
	if result[path] == nil || result[path].incomplete || result[path].info == nil {
		t.Fatal("native isolated type loading failed")
	}
	if _, err := os.Stat(filepath.Join(dir, "driver-observed")); !os.IsNotExist(err) {
		t.Fatal("off invoked custom driver")
	}
}
