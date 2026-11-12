package plans

import "testing"

// Every toolbar icon is a whole 16 × 16 grid of known inks.
func TestUtilityIconsAreWholePixelGrids(t *testing.T) {
	for name, rows := range utilityIcons {
		if len(rows) != 16 {
			t.Errorf("%s has %d rows", name, len(rows))
		}
		for i, row := range rows {
			if len(row) != 16 {
				t.Errorf("%s row %d is %d wide", name, i, len(row))
			}
			for _, c := range row {
				if c != '.' && c != '#' && c != '+' && c != 't' && c != 'T' && c != 'r' && c != 'R' && c != 'w' {
					t.Errorf("%s row %d has ink %q", name, i, c)
				}
			}
		}
	}
}
