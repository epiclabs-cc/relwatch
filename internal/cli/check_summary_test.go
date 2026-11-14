package cli

import "testing"

func TestCheckSummary(t *testing.T) {
	cases := []struct {
		name   string
		counts []checkCount
		want   string
	}{
		{
			"all kinds",
			[]checkCount{{"project(s)", 1, 1}, {"image(s)", 1, 1}, {"feed(s)", 1, 1}},
			"Checked 1 of 1 project(s), 1 of 1 image(s), 1 of 1 feed(s)",
		},
		{
			"feeds only",
			[]checkCount{{"project(s)", 0, 0}, {"image(s)", 0, 0}, {"feed(s)", 2, 2}},
			"Checked 2 of 2 feed(s)",
		},
		{
			"partial failure",
			[]checkCount{{"project(s)", 2, 3}, {"image(s)", 0, 1}, {"feed(s)", 0, 0}},
			"Checked 2 of 3 project(s), 0 of 1 image(s)",
		},
		{
			"nothing tracked",
			[]checkCount{{"project(s)", 0, 0}, {"image(s)", 0, 0}, {"feed(s)", 0, 0}},
			"Checked nothing",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := checkSummary(tc.counts...); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
