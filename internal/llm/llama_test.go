package llm

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestLlamaEngine_ListModels(t *testing.T) {
	// Create a temp directory for models
	tempDir, err := os.MkdirTemp("", "models")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create dummy model files
	files := []string{"model1.gguf", "model2.gguf", "readme.txt", "model3.bin"}
	for _, f := range files {
		path := filepath.Join(tempDir, f)
		if err := os.WriteFile(path, []byte("dummy content"), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
	}

	// Create engine with model path in temp dir
	// We don't need to actually load the model, just set the path
	engine := &LlamaEngine{
		modelPath: filepath.Join(tempDir, "model1.gguf"),
	}

	// List models
	models, err := engine.ListModels()
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	// Verify results
	expected := []string{"model1.gguf", "model2.gguf"}
	sort.Strings(models)
	sort.Strings(expected)

	if len(models) != len(expected) {
		t.Errorf("Expected %d models, got %d", len(expected), len(models))
	}

	for i, m := range models {
		if m != expected[i] {
			t.Errorf("Expected model %s, got %s", expected[i], m)
		}
	}
}

func TestLlamaEngine_GetModelPath(t *testing.T) {
	path := "/path/to/model.gguf"
	engine := &LlamaEngine{
		modelPath: path,
	}
	if engine.GetModelPath() != path {
		t.Errorf("Expected path %s, got %s", path, engine.GetModelPath())
	}
}
