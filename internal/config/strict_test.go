package config

import (
	"strings"
	"testing"
)

type sampleConfig struct {
	Name string `toml:"name"`
	Port int    `toml:"port"`
}

func TestDecodeStrictBytesPassesValidConfig(t *testing.T) {
	var cfg sampleConfig
	err := decodeStrictBytes("test", []byte(`name = "api"`+"\n"+`port = 8080`), &cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "api" || cfg.Port != 8080 {
		t.Errorf("unexpected decoded values: %+v", cfg)
	}
}

func TestDecodeStrictBytesRejectsUnknownKey(t *testing.T) {
	var cfg sampleConfig
	err := decodeStrictBytes("test", []byte(`name = "api"`+"\n"+`unknown = 42`), &cfg)
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
	if !strings.Contains(err.Error(), "unknown keys") || !strings.Contains(err.Error(), "unknown") {
		t.Errorf("error message should mention the unknown key, got: %v", err)
	}
}

func TestDecodeStrictBytesRejectsUnknownSection(t *testing.T) {
	var cfg sampleConfig
	err := decodeStrictBytes("test", []byte(`name = "api"`+"\n\n"+`[extras]`+"\n"+`color = "red"`), &cfg)
	if err == nil {
		t.Fatal("expected error for unknown section")
	}
	if !strings.Contains(err.Error(), "extras") {
		t.Errorf("error message should mention the unknown section, got: %v", err)
	}
}

func TestDecodeStrictBytesSurfacesParseErrorAsIs(t *testing.T) {
	var cfg sampleConfig
	err := decodeStrictBytes("test", []byte(`not = valid = toml`), &cfg)
	if err == nil {
		t.Fatal("expected parse error")
	}
}
