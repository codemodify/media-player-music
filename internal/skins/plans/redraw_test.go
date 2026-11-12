package plans

import (
	"encoding/json"
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

// productNames are names no skin of the app's may carry: in its ID, its
// layouts, sheets or sprites, anywhere in its file but its label. The labels
// are the ones the skins were shown under before the redraw (the owner,
// 2026-11-13), pinned by TestEverySkinKeepsItsOldLabel.
var productNames = regexp.MustCompile(`(?i)winamp|\bwindows\b|media\s*player|\bwmp\b|xbox|half[\s-]*life|valve|itunes|apple|mac\s*os|aqua|realplayer|realnetworks|\bvlc\b|videolan|\bkde\b|amarok|quake|\bsony\b|\bti-?\d|texas instruments|microsoft|\bdosamp\b|ms-dos|headspace`)

func TestNoProductNamesInAnySkin(t *testing.T) {
	for _, p := range Plans() {
		doc, err := skingen.Manifest(p)
		if err != nil {
			t.Fatal(err)
		}
		var manifest map[string]any
		if err := json.Unmarshal(doc, &manifest); err != nil {
			t.Fatal(err)
		}
		if manifest["label"] != p.Label {
			t.Errorf("%s's skin.json is labelled %q, its plan %q", p.Name, manifest["label"], p.Label)
		}
		delete(manifest, "label")
		rest, err := json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		if m := productNames.FindAll(rest, -1); len(m) > 0 {
			t.Errorf("%s's skin.json names %q outside its label", p.Name, m)
		}
		if productNames.MatchString(p.Name) {
			t.Errorf("%s's ID names a product", p.Name)
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

// Every skin carries the label it was shown under before the redraw (the
// owner, 2026-11-13); only its ID is new.
func TestEverySkinKeepsItsOldLabel(t *testing.T) {
	want := map[string]string{
		"slate":     "Winamp",
		"silver":    "Winamp Silver",
		"dossier":   "Winamp Classified",
		"textmode":  "DOSamp",
		"bevel":     "Windows 98",
		"amber":     "Winamp Classic Amber",
		"tally":     "Calculator",
		"headunit":  "Sony CDX-MP3",
		"graphing":  "TI-82",
		"radar":     "Radar Amp",
		"forge":     "Quake 3 Arena",
		"wingspan":  "Win Media Player - Headspace",
		"pod":       "Win Media Player - Xbox",
		"gauge":     "Win Media Player - Half Life 2",
		"fandom":    "Win Media Player - Fandom",
		"aluminium": "Mac OS X - iTunes",
		"cobalt":    "Win Media Player - Series 9",
		"utility":   "VLC - Classic",
		"console":   "RealPlayer - Classic",
		"sidebar":   "KDE - Amarok 1.4",
		"fancy":     "Fancy",
	}
	seen := map[string]bool{}
	for _, p := range Plans() {
		seen[p.Name] = true
		if label, ok := want[p.Name]; !ok {
			t.Errorf("%s: no label pinned", p.Name)
		} else if p.Label != label {
			t.Errorf("%s is labelled %q, want %q", p.Name, p.Label, label)
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("no plan %s", name)
		}
	}
}

// goProductNames are the products the skins were once named for, as they
// show in Go names: the long ones anywhere in a name (its words run
// together, lower case), the short ones only at the start of one of its
// words (so FlexBox or svLCD are not taken for one).
var (
	goProductNamesAnywhere = regexp.MustCompile(`winamp|halflife|itunes|realplayer|amarok|series9|headspace|quake|windows98`)
	goProductNamesAtAWord  = regexp.MustCompile(`(^|_)(xbox|vlc|sony|ti_?82|wmp)`)
)

// goNameNamesAProduct says whether a Go name (an identifier, a package
// path's folder, a file's name) names one of goProductNames.
func goNameNamesAProduct(name string) bool {
	var words []string
	runes := []rune(name)
	start := 0
	kind := func(r rune) int {
		switch {
		case r >= 'A' && r <= 'Z':
			return 1
		case r >= '0' && r <= '9':
			return 2
		case r == '_' || r == '-' || r == '.' || r == '/':
			return 3
		}
		return 0
	}
	for i := 1; i <= len(runes); i++ {
		cut := i == len(runes)
		if !cut {
			was, now := kind(runes[i-1]), kind(runes[i])
			next := -1
			if i+1 < len(runes) {
				next = kind(runes[i+1])
			}
			// lower to upper, letter to digit and back, a separator, and
			// the last capital of a run before a lower-case letter (LCDWell)
			cut = was == 0 && now == 1 || (was == 2) != (now == 2) || was == 3 || now == 3 ||
				was == 1 && now == 1 && next == 0
		}
		if cut {
			if w := strings.Trim(string(runes[start:i]), "_-./"); w != "" {
				words = append(words, strings.ToLower(w))
			}
			start = i
		}
	}
	return goProductNamesAnywhere.MatchString(strings.Join(words, "")) || goProductNamesAtAWord.MatchString(strings.Join(words, "_"))
}

func TestGoNameNamesAProduct(t *testing.T) {
	for name, want := range map[string]bool{
		"xboxTransparent": true, "TestVLCActions": true, "TestHalfLife2Skin": true, "halflife2": true, "iTunesModes": true,
		"TestITunesSkin": true, "series9skin": true, "RealPlayerSkin": true, "TI82Skin": true, "Windows98Skin": true,
		"winamp_silver.go": true, "TestWMPCatalogue": true, "SonyCDXMP3Skin": true, "Quake3ArenaSkin": true,
		"FlexBox": false, "example.com/skins/vlc": true, "svLCDWell": false, "LCD": false, "PodSkin": false, "slate.go": false, "utility": false, "Timeline82": false,
	} {
		if got := goNameNamesAProduct(name); got != want {
			t.Errorf("%s: %v, want %v", name, got, want)
		}
	}
}

// No Go name of the app's names a product the skins were once named for:
// no package path or file name, no identifier (a package, a type, a
// function, a test, a variable, an import's name). The originals and the
// old plans that cut them (internal/skins/sources) await the owner's
// history rewrite and are left out.
func TestNoGoNameNamesAProduct(t *testing.T) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	files := 0
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := rel(root, path)
		if entry.IsDir() {
			switch {
			case path == root:
				return nil
			case strings.HasPrefix(entry.Name(), "."), name == filepath.Join("internal", "skins", "sources"),
				entry.Name() == "node_modules", name == "out", name == "shots", name == "bin":
				return filepath.SkipDir
			}
			if goNameNamesAProduct(entry.Name()) {
				// a folder of Go files is a package path
				if gos, _ := filepath.Glob(filepath.Join(path, "*.go")); len(gos) > 0 {
					t.Errorf("the package %s names a product", name)
				}
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		files++
		if goNameNamesAProduct(entry.Name()) {
			t.Errorf("the file %s names a product", name)
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			if p, _ := strconv.Unquote(spec.Path.Value); goNameNamesAProduct(p) {
				t.Errorf("%s imports %s", fset.Position(spec.Pos()), p)
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && goNameNamesAProduct(id.Name) {
				t.Errorf("%s: %s", fset.Position(id.Pos()), id.Name)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files < 100 {
		t.Fatalf("only %d Go files read from %s", files, root)
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
