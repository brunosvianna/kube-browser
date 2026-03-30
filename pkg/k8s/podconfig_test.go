package k8s

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestParseKeyValuePairs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{
			name:  "empty string returns nil",
			input: "",
			want:  nil,
		},
		{
			name:  "whitespace-only returns nil",
			input: "   ",
			want:  nil,
		},
		{
			name:  "single pair",
			input: "disk=ssd",
			want:  map[string]string{"disk": "ssd"},
		},
		{
			name:  "multiple pairs",
			input: "disk=ssd,zone=us-east-1a",
			want:  map[string]string{"disk": "ssd", "zone": "us-east-1a"},
		},
		{
			name:  "pairs with spaces around delimiters",
			input: " disk = ssd , zone = us-east-1a ",
			want:  map[string]string{"disk": "ssd", "zone": "us-east-1a"},
		},
		{
			name:  "value with equals sign",
			input: "label=kubernetes.io/arch=amd64",
			want:  map[string]string{"label": "kubernetes.io/arch=amd64"},
		},
		{
			name:  "json object",
			input: `{"disk":"ssd","zone":"us-east-1a"}`,
			want:  map[string]string{"disk": "ssd", "zone": "us-east-1a"},
		},
		{
			name:  "json object with single key",
			input: `{"environment":"production"}`,
			want:  map[string]string{"environment": "production"},
		},
		{
			name:  "invalid json returns nil",
			input: `{"bad":}`,
			want:  nil,
		},
		{
			name:  "invalid pair (no equals) skipped",
			input: "good=value,badinput,other=ok",
			want:  map[string]string{"good": "value", "other": "ok"},
		},
		{
			name:  "all pairs invalid returns nil",
			input: "noequalssign",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseKeyValuePairs(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseKeyValuePairs(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseTolerations(t *testing.T) {
	effectNoSchedule := corev1.TaintEffectNoSchedule
	effectNoExecute := corev1.TaintEffectNoExecute
	opExists := corev1.TolerationOpExists
	opEqual := corev1.TolerationOpEqual

	tests := []struct {
		name  string
		input string
		want  []corev1.Toleration
	}{
		{
			name:  "empty string returns nil",
			input: "",
			want:  nil,
		},
		{
			name:  "whitespace-only returns nil",
			input: "   ",
			want:  nil,
		},
		{
			name:  "invalid json returns nil",
			input: "not-json",
			want:  nil,
		},
		{
			name:  "empty json array returns empty slice",
			input: "[]",
			want:  []corev1.Toleration{},
		},
		{
			name:  "single toleration with key and effect",
			input: `[{"key":"dedicated","operator":"Exists","effect":"NoSchedule"}]`,
			want: []corev1.Toleration{
				{Key: "dedicated", Operator: opExists, Effect: effectNoSchedule},
			},
		},
		{
			name:  "multiple tolerations",
			input: `[{"key":"gpu","operator":"Equal","value":"true","effect":"NoSchedule"},{"key":"spot","effect":"NoExecute"}]`,
			want: []corev1.Toleration{
				{Key: "gpu", Operator: opEqual, Value: "true", Effect: effectNoSchedule},
				{Key: "spot", Effect: effectNoExecute},
			},
		},
		{
			name:  "toleration with tolerationSeconds",
			input: `[{"key":"node.kubernetes.io/not-ready","operator":"Exists","effect":"NoExecute","tolerationSeconds":300}]`,
			want: func() []corev1.Toleration {
				secs := int64(300)
				return []corev1.Toleration{
					{Key: "node.kubernetes.io/not-ready", Operator: opExists, Effect: effectNoExecute, TolerationSeconds: &secs},
				}
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTolerations(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTolerations(%q)\ngot  %+v\nwant %+v", tt.input, got, tt.want)
			}
		})
	}
}
