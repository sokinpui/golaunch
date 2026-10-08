package launchd

import (
	"bytes"
	"fmt"
	"os"

	"howett.net/plist"
)

func EncodePlist(cfg PlistConfig) ([]byte, error) {
	var buf bytes.Buffer
	encoder := plist.NewEncoderForFormat(&buf, plist.XMLFormat)
	encoder.Indent("\t")
	if err := encoder.Encode(cfg); err != nil {
		return nil, fmt.Errorf("failed to encode plist: %w", err)
	}
	return buf.Bytes(), nil
}

func DecodePlist(data []byte) (*PlistConfig, error) {
	var cfg PlistConfig
	decoder := plist.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode plist: %w", err)
	}
	return &cfg, nil
}

func WritePlistFile(path string, cfg PlistConfig) error {
	data, err := EncodePlist(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func ReadPlistFile(path string) (*PlistConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return DecodePlist(data)
}
