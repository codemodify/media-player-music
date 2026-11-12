package plans

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"goframework.io/desktop-ui-skingen"
)

const module = "github.com/codemodify/media-player-music/"

// The generator draws every skin from code: neither it nor any of the
// app's packages it imports opens a file, and none of them is, or names,
// anything under internal/skins/sources (the originals, kept until the
// owner's history rewrite).
func TestGeneratorReadsNoOriginals(t *testing.T) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var walk func(pkg string)
	walk = func(pkg string) {
		if seen[pkg] {
			return
		}
		seen[pkg] = true
		dir := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(pkg, module)))
		if strings.Contains(pkg, "/internal/skins/sources") {
			t.Errorf("the generator imports %s", pkg)
			return
		}
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil || len(files) == 0 {
			t.Fatalf("%s: no Go files (%v)", pkg, err)
		}
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.ImportsOnly|parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			if ignored(file) {
				continue
			}
			for _, imp := range file.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				switch path {
				case "os", "io/fs", "io/ioutil", "path/filepath":
					// The generator's own command writes the packs; nothing
					// the plans import may read.
					if pkg != module+"internal/skins/cmd/generate" {
						t.Errorf("%s imports %s", rel(root, f), path)
					}
				}
				if strings.HasPrefix(path, module) {
					walk(path)
				}
			}
			whole, err := parser.ParseFile(token.NewFileSet(), f, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(whole, func(n ast.Node) bool {
				if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && strings.Contains(lit.Value, "sources/") {
					t.Errorf("%s names %s", rel(root, f), lit.Value)
				}
				return true
			})
		}
	}
	walk(module + "internal/skins/cmd/generate")
	if !seen[module+"internal/skins/plans"] {
		t.Fatal("the generator does not import the plans")
	}
}

func ignored(f *ast.File) bool {
	for _, c := range f.Comments {
		for _, l := range c.List {
			if strings.HasPrefix(l.Text, "//go:build") && strings.Contains(l.Text, "ignore") {
				return true
			}
		}
	}
	return false
}

func rel(root, f string) string {
	r, _ := filepath.Rel(root, f)
	return r
}

// productNames are names no skin of the app's may carry: in its file, its
// label, its layouts, sheets or sprites.
var productNames = regexp.MustCompile(`(?i)winamp|\bwindows\b|media\s*player|\bwmp\b|xbox|half[\s-]*life|valve|itunes|apple|mac\s*os|aqua|realplayer|realnetworks|\bvlc\b|videolan|\bkde\b|amarok|quake|\bsony\b|\bti-?\d|texas instruments|microsoft|\bdosamp\b|ms-dos|headspace`)

func TestNoProductNamesInAnySkin(t *testing.T) {
	for _, p := range Plans() {
		doc, err := skingen.Manifest(p)
		if err != nil {
			t.Fatal(err)
		}
		if m := productNames.FindAll(doc, -1); len(m) > 0 {
			t.Errorf("%s's skin.json names %q", p.Name, m)
		}
		if productNames.MatchString(p.Name + " " + p.Label) {
			t.Errorf("%s is named %q", p.Name, p.Label)
		}
	}
	// and the geometry tables the redrawn skins are built on
	files, _ := filepath.Glob("geometry_*.go")
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		body := string(data)
		if i := strings.Index(body, "package plans"); i >= 0 {
			body = body[i:] // the header says which old pack it came from
		}
		if m := productNames.FindAllString(body, -1); len(m) > 0 {
			t.Errorf("%s names %q", f, m)
		}
	}
}

// Every sprite of every redrawn skin is drawn: something lands in its cell
// (but for the space in a glyph set).
func TestEveryRedrawnSpriteIsDrawn(t *testing.T) {
	for _, p := range Plans() {
		switch p.Name {
		case "slate", "silver", "fandom", "fancy":
			continue
		}
		t.Run(p.Name, func(t *testing.T) {
			for _, sh := range p.Sheets {
				img := skingen.Render(sh, 1)
				for _, c := range sh.Cells {
					if strings.HasSuffix(c.Name, "font.20") {
						continue
					}
					ink := 0
					for y := c.Y; y < c.Y+c.H; y++ {
						for x := c.X; x < c.X+c.W; x++ {
							if img.Pix[(y*img.Width+x)*4+3] > 0 {
								ink++
							}
						}
					}
					if ink == 0 {
						t.Errorf("%s draws nothing", c.Name)
					}
				}
			}
		})
	}
}

func testCell(t *testing.T, p *Plan, name string) Cell {
	t.Helper()
	for _, sh := range p.Sheets {
		for _, c := range sh.Cells {
			if c.Name == name {
				c.X, c.Y = 0, 0
				return c
			}
		}
	}
	t.Fatalf("missing sprite %s", name)
	return Cell{}
}
