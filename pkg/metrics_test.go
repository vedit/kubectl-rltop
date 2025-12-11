package pkg

import (
	"testing"
)

func TestFormatCPU(t *testing.T) {
	tests := []struct {
		name      string
		millicores int64
		want      string
	}{
		{"zero", 0, "0"},
		{"millicores", 100, "100m"},
		{"one core", 1000, "1"},
		{"fractional cores", 1500, "1.50"},
		{"large value", 5000, "5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatCPU(tt.millicores)
			if got != tt.want {
				t.Errorf("formatCPU(%d) = %v, want %v", tt.millicores, got, tt.want)
			}
		})
	}
}

func TestFormatMemory(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero", 0, "0"},
		{"bytes", 512, "512B"},
		{"kilobytes", 1024, "1.00Ki"},
		{"megabytes", 1048576, "1.00Mi"},
		{"gigabytes", 1073741824, "1.00Gi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatMemory(tt.bytes)
			if got != tt.want {
				t.Errorf("formatMemory(%d) = %v, want %v", tt.bytes, got, tt.want)
			}
		})
	}
}

