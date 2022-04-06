package ipfs

import "testing"

func TestIs(t *testing.T) {
	tests := []struct {
		name string
		link string
		want bool
	}{
		{
			name: "ipfs://QmWYTUjkRusrhz4BCmoMKfSA8DBXk5pH2oAN83S9mABE3w",
			link: "ipfs://QmWYTUjkRusrhz4BCmoMKfSA8DBXk5pH2oAN83S9mABE3w",
			want: true,
		}, {
			name: "ipfs://zdj7WkPvrxL7VxiWbjBP5rfshPtAzXwZ77uvZhfSAoHDeb3iw/1",
			link: "ipfs://zdj7WkPvrxL7VxiWbjBP5rfshPtAzXwZ77uvZhfSAoHDeb3iw/1",
			want: true,
		}, {
			name: "ipfs://bafkreie7cvrfe6cgiat6nrffmtlf5al4fkae6hxtoy7lfebbz62v32lyvi",
			link: "ipfs://bafkreie7cvrfe6cgiat6nrffmtlf5al4fkae6hxtoy7lfebbz62v32lyvi",
			want: true,
		}, {
			name: "invalid",
			link: "ipfs://invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Is(tt.link); got != tt.want {
				t.Errorf("Is() = %v, want %v", got, tt.want)
			}
		})
	}
}
