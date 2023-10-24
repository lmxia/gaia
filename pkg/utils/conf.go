package utils

import (
	"bufio"
	"io"
	"os"
	"strings"

	"k8s.io/klog/v2"
)

// InitConfig returns the map and the keys list of a configMap file.
// The file content is in yaml-like format with colons(":") between key and value.
func InitConfig(path string) (configMap map[string]string, mapKeyList []string, err error) {
	configMap = make(map[string]string)
	f, err := os.Open(path)
	if err != nil {
		klog.Errorf("The file(%v) not exits.", path)
		return nil, nil, err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	for {
		b, _, err := r.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, nil, err
		}
		s := strings.TrimSpace(string(b))
		index := strings.Index(s, ":")
		if index < 0 {
			continue
		}
		key := strings.TrimSpace(s[:index])
		if len(key) == 0 {
			continue
		}
		value := strings.TrimSpace(s[index+1:])
		if len(value) == 0 {
			continue
		}
		configMap[key] = value
		mapKeyList = append(mapKeyList, key)
	}
	return configMap, mapKeyList, nil
}
