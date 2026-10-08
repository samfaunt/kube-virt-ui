package image

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
)

func TestValidate(t *testing.T) {
	for name, r := range map[string]CreateRequest{
		"bad name": {Name: "My ISO", Type: TypeISO, Size: "5Gi"},
		"bad type": {Name: "a", Type: "exe", Size: "5Gi"},
		"tiny":     {Name: "a", Type: TypeISO, Size: "1Mi"},
		"bad size": {Name: "a", Type: TypeDisk, Size: "big"},
	} {
		if err := r.validate(); !IsValidation(err) {
			t.Errorf("%s: %v", name, err)
		}
	}
	r := CreateRequest{Name: "fedora-iso", Type: TypeISO, Size: "2048Mi"}
	if err := r.validate(); err != nil || r.Size != "2Gi" {
		t.Fatalf("%v %+v", err, r)
	}
}

func TestSummarize(t *testing.T) {
	dv := &unstructured.Unstructured{}
	json.Unmarshal([]byte(`{"metadata":{"name":"iso","annotations":{"kubevirt-ui.io/image-type":"iso"}},
		"spec":{"storage":{"resources":{"requests":{"storage":"5Gi"}}}},
		"status":{"phase":"UploadReady","progress":"N/A","conditions":[{"type":"Running","status":"True","reason":"Pod is running"}]}}`), &dv.Object)
	img := summarize(dv)
	if img.Type != "iso" || img.Size != "5Gi" || img.Phase != "UploadReady" || img.Progress != "" || img.Problem != "" {
		t.Fatalf("%+v", img)
	}
}
