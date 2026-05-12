package numfmt

import "testing"

func TestF(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0.812345, "0.812"},
		{10.24, "10.24"},
		{542, "542"},
		{13.0, "13"},
		{-4.05, "-4.05"},
		{0, "0"},
		{0.001, "0.001"},
		{0.0001, "0"},   // 第 4 位四捨五入到 0
		{0.0005, "0.001"}, // 邊界：四捨五入
		{-0.812, "-0.812"},
		{88.62, "88.62"},
		{1.155, "1.155"},
	}
	for _, c := range cases {
		got := F(c.in)
		if got != c.want {
			t.Errorf("F(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
