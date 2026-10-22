package review

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const MaxDiffBytes = 1 << 20
const MaxDiffFiles = 4096
const MaxDiffHunks = 10000

var errCIInput = errors.New("PUBLIC_INPUT: invalid review comparison input")

type sourceTree map[string][]byte

type ciCapture struct {
	inputs map[string][]byte
	valid  bool
	// Only cross-session external comparison uses this private loader evidence.
	// All captured bytes still participate in same-session mutation checks.
	unusedTestMain map[string]bool
}

// readTree bounds all context, including non-review files. No symlinks or special
// files are followed. OpenRoot confines reads even if a directory is replaced.
func readTree(ctx context.Context, root string) (sourceTree, error) {
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, errCIInput
	}
	defer r.Close()
	out := sourceTree{}
	total, directories := 0, 0
	err = fs.WalkDir(r.FS(), ".", func(p string, d fs.DirEntry, e error) error {
		if e != nil || ctx.Err() != nil {
			return errCIInput
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".codex", ".agents", ".aws", ".gocache", "vendor", "node_modules", "dist", "build", "coverage":
				return fs.SkipDir
			}
			directories++
			if directories > maxTypeFiles {
				return errCIInput
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return errCIInput
		}
		info, e := d.Info()
		if e != nil || !info.Mode().IsRegular() || info.Size() > maxTypeFileBytes || len(out) >= maxTypeFiles {
			return errCIInput
		}
		f, e := r.Open(p)
		if e != nil {
			return errCIInput
		}
		b, e := io.ReadAll(io.LimitReader(f, maxTypeFileBytes+1))
		f.Close()
		total += len(b)
		if e != nil || len(b) > maxTypeFileBytes || total > maxTypeInputBytes {
			return errCIInput
		}
		out[p] = b
		return nil
	})
	if err != nil {
		return nil, errCIInput
	}
	return out, nil
}
func equalTrees(a, b sourceTree) bool {
	if len(a) != len(b) {
		return false
	}
	for p, v := range a {
		w, ok := b[p]
		if !ok || !bytes.Equal(v, w) {
			return false
		}
	}
	return true
}
func exactPath(p string) bool {
	return p != "" && p != "." && utf8.ValidString(p) && path.Clean(p) == p && !strings.HasPrefix(p, "/") && p != ".." && !strings.HasPrefix(p, "../") && !strings.ContainsAny(p, "\\:*?[]\x00\r\n\t")
}
func relativeFile(root, file string) string {
	abs, e := filepath.Abs(file)
	if e != nil {
		return ""
	}
	p, e := filepath.Rel(root, abs)
	p = filepath.ToSlash(p)
	if e != nil || !exactPath(p) {
		return ""
	}
	return p
}

type diffFile struct {
	old, new string
	lines    map[int]bool
	kind     string
}
type diffSelection struct {
	files  []diffFile
	valid  bool
	counts DiffCounts
}

// DiffCounts counts file records, except Invalid/Unknown, which count input or
// source correspondence problems. These are classifications, not trust evidence.
type DiffCounts struct {
	Modified   int `json:"modified"`
	Added      int `json:"added"`
	Deleted    int `json:"deleted"`
	Renamed    int `json:"renamed"`
	Binary     int `json:"binary"`
	Invalid    int `json:"invalid"`
	Unknown    int `json:"unknown"`
	Unlisted   int `json:"unlisted"`
	ReadFailed int `json:"read_failed"`
}

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(?: .*)?$`)

func textLines(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	a := strings.Split(string(b), "\n")
	if a[len(a)-1] == "" {
		a = a[:len(a)-1]
	}
	return a
}
func diffPath(s, prefix string) (string, bool) {
	if s == "/dev/null" {
		return "", true
	}
	if !strings.HasPrefix(s, prefix) {
		return "", false
	}
	s = strings.TrimPrefix(s, prefix)
	return s, exactPath(s)
}

// parseDiff accepts Git text unified patches, including rename metadata. It
// verifies the entire reconstructed result, not merely the advertised hunk lines.
func parseDiff(b []byte, old, head sourceTree) diffSelection {
	out := diffSelection{valid: true}
	bad := func() diffSelection { out.valid = false; out.counts.Invalid++; return out }
	if len(b) > MaxDiffBytes || !utf8.Valid(b) || bytes.IndexByte(b, 0) >= 0 {
		return bad()
	}
	lines := strings.Split(string(b), "\n")
	i, hunks := 0, 0
	seen := map[string]bool{}
	seenOld := map[string]bool{}
	for i < len(lines) {
		if lines[i] == "" {
			i++
			continue
		}
		if !strings.HasPrefix(lines[i], "diff --git ") {
			return bad()
		}
		fields := strings.Fields(lines[i])
		if len(fields) != 4 {
			return bad()
		}
		op, ok := diffPath(fields[2], "a/")
		np, ok2 := diffPath(fields[3], "b/")
		if !ok || !ok2 || op == "" || np == "" {
			return bad()
		}
		i++
		f := diffFile{old: op, new: np, lines: map[int]bool{}, kind: "modified"}
		binary, rename := false, false
		renameFrom, renameTo := false, false
		for i < len(lines) && !strings.HasPrefix(lines[i], "--- ") && !strings.HasPrefix(lines[i], "diff --git ") {
			l := lines[i]
			switch {
			case strings.HasPrefix(l, "Binary files "):
				binary = true
			case strings.HasPrefix(l, "rename from "):
				if renameFrom || strings.TrimPrefix(l, "rename from ") != op {
					return bad()
				}
				rename = true
				renameFrom = true
			case strings.HasPrefix(l, "rename to "):
				if renameTo || strings.TrimPrefix(l, "rename to ") != np {
					return bad()
				}
				rename = true
				renameTo = true
			case strings.HasPrefix(l, "index "), strings.HasPrefix(l, "similarity index "), strings.HasPrefix(l, "old mode "), strings.HasPrefix(l, "new mode "), strings.HasPrefix(l, "new file mode "), strings.HasPrefix(l, "deleted file mode "), l == "":
			default:
				return bad()
			}
			i++
		}
		if op != np && !rename {
			return bad()
		}
		if rename && (!renameFrom || !renameTo || op == np || head[op] != nil || old[np] != nil) {
			return bad()
		}
		if binary {
			f.kind = "binary"
			out.counts.Binary++
			out.counts.Unknown++
		} else if i < len(lines) && strings.HasPrefix(lines[i], "--- ") {
			a, ok := diffPath(strings.TrimPrefix(lines[i], "--- "), "a/")
			i++
			if i >= len(lines) || !strings.HasPrefix(lines[i], "+++ ") {
				return bad()
			}
			z, ok2 := diffPath(strings.TrimPrefix(lines[i], "+++ "), "b/")
			i++
			if !ok || !ok2 || (a != "" && a != op) || (z != "" && z != np) || (a == "" && z == "") {
				return bad()
			}
			f.old, f.new = a, z
			ob, oexists := old[a]
			nb, nexists := head[z]
			if (a != "" && !oexists) || (z != "" && !nexists) || (a == "" && old[z] != nil) || (z == "" && head[a] != nil) {
				return bad()
			}
			// This finite text subset requires LF-terminated nonempty files.
			// Refuse rather than equate sources differing only in the final newline.
			if (len(ob) > 0 && ob[len(ob)-1] != '\n') || (len(nb) > 0 && nb[len(nb)-1] != '\n') {
				return bad()
			}
			ol, nl := textLines(ob), textLines(nb)
			oc, nc := 0, 0
			reconstructed := []string{}
			for i < len(lines) && strings.HasPrefix(lines[i], "@@ ") {
				hunks++
				if hunks > MaxDiffHunks {
					return bad()
				}
				m := hunkHeader.FindStringSubmatch(lines[i])
				if m == nil {
					return bad()
				}
				i++
				startO, e1 := strconv.Atoi(m[1])
				startN, e2 := strconv.Atoi(m[3])
				countO, countN := 1, 1
				var e3, e4 error
				if m[2] != "" {
					countO, e3 = strconv.Atoi(m[2])
				}
				if m[4] != "" {
					countN, e4 = strconv.Atoi(m[4])
				}
				if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
					return bad()
				}
				oi, ni := startO, startN
				if countO > 0 {
					oi--
				}
				if countN > 0 {
					ni--
				}
				if oi < oc || ni < nc || oi > len(ol) || ni > len(nl) || oi-oc != ni-nc {
					return bad()
				}
				for oc < oi {
					if ol[oc] != nl[nc] {
						return bad()
					}
					reconstructed = append(reconstructed, ol[oc])
					oc++
					nc++
				}
				ro, rn := 0, 0
				for i < len(lines) && (ro < countO || rn < countN) {
					l := lines[i]
					i++
					if l == "\\ No newline at end of file" {
						return bad()
					}
					if len(l) == 0 {
						return bad()
					}
					switch l[0] {
					case ' ':
						if oc >= len(ol) || nc >= len(nl) || ol[oc] != l[1:] || nl[nc] != l[1:] {
							return bad()
						}
						oc++
						nc++
						ro++
						rn++
						reconstructed = append(reconstructed, l[1:])
					case '-':
						if oc >= len(ol) || ol[oc] != l[1:] {
							return bad()
						}
						oc++
						ro++
						f.lines[max(1, nc+1)] = true
					case '+':
						if nc >= len(nl) || nl[nc] != l[1:] {
							return bad()
						}
						nc++
						rn++
						f.lines[nc] = true
						reconstructed = append(reconstructed, l[1:])
					default:
						return bad()
					}
					if ro > countO || rn > countN {
						return bad()
					}
				}
				if ro != countO || rn != countN {
					return bad()
				}
				if i < len(lines) && lines[i] == "\\ No newline at end of file" {
					return bad()
				}
			}
			for oc < len(ol) {
				if nc >= len(nl) || ol[oc] != nl[nc] {
					return bad()
				}
				reconstructed = append(reconstructed, ol[oc])
				oc++
				nc++
			}
			if nc != len(nl) || strings.Join(reconstructed, "\n") != strings.Join(nl, "\n") {
				return bad()
			}
			if a == "" {
				f.kind = "added"
			} else if z == "" {
				f.kind = "deleted"
			} else if a != z {
				if !rename {
					return bad()
				}
				f.kind = "renamed"
			}
		} else {
			if !bytes.Equal(old[op], head[np]) || old[op] == nil || head[np] == nil {
				return bad()
			}
			if rename {
				f.kind = "renamed"
			}
		}
		key := f.new
		if key == "" {
			key = f.old
		}
		if seen[key] || (f.old != "" && seenOld[f.old]) || len(out.files) >= MaxDiffFiles {
			return bad()
		}
		seen[key] = true
		if f.old != "" {
			seenOld[f.old] = true
		}
		switch f.kind {
		case "modified":
			out.counts.Modified++
		case "added":
			out.counts.Added++
		case "deleted":
			out.counts.Deleted++
		case "renamed":
			out.counts.Renamed++
		}
		out.files = append(out.files, f)
	}
	// A patch cannot hide source changes by omitting a file. There is no Git index
	// here: new/unlisted files are classified without claiming tracked status.
	covered := map[string]bool{}
	for _, f := range out.files {
		covered[f.old] = true
		covered[f.new] = true
	}
	for p, v := range old {
		w, ok := head[p]
		if (!ok || !bytes.Equal(v, w)) && !covered[p] {
			out.counts.Unlisted++
		}
	}
	for p := range head {
		if _, ok := old[p]; !ok && !covered[p] {
			out.counts.Unlisted++
		}
	}
	if out.counts.Unlisted > 0 || out.counts.Unknown > 0 {
		out.valid = false
		out.counts.Unknown++
	}
	return out
}
