package pkg

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
)

func TestFormatResourceQuantity(t *testing.T) {
	tests := []struct {
		name     string
		quantity resource.Quantity
		isCPU    bool
		want     string
	}{
		{"zero CPU", resource.Quantity{}, true, "<none>"},
		{"zero memory", resource.Quantity{}, false, "<none>"},
		{"CPU millicores", resource.MustParse("100m"), true, "100m"},
		{"CPU cores", resource.MustParse("2"), true, "2"},
		{"memory bytes", resource.MustParse("1024"), false, "1024"},
		{"memory Mi", resource.MustParse("128Mi"), false, "128Mi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatResourceQuantity(tt.quantity, tt.isCPU)
			if got != tt.want {
				t.Errorf("formatResourceQuantity(%v, %v) = %v, want %v", tt.quantity, tt.isCPU, got, tt.want)
			}
		})
	}
}

