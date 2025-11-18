package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func StringToInt(value string) int {
	newValue, err := strconv.Atoi(value)
	if err != nil {
		newValue = 0
	}
	return newValue
}
func StringStartWith(s, prefix string) bool {
	return strings.HasPrefix(s, prefix)
}

func saveToFile(id, body string) error {
	f, err := os.Create(id)
	if err != nil {
		log.Errorf("Невозможно создать файл для записи последнего id  объекта GLPI")
	}
	defer func(f *os.File) {
		_ = f.Close()

	}(f)
	_, _ = f.WriteString(body)
	return nil
}
func WalkFiles(root, pattern string) ([]string, error) {
	var matches []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if matched, err := filepath.Match(pattern, filepath.Base(path)); err != nil {
			return err
		} else if matched {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matches, nil
}

const (
	statusCritical = "critical"
	statusInfo     = "info"
	statusSuccess  = "success"
	statusWarning  = "warning"
)
const (
	iconComplate = ":ok3:"
	iconWorking  = ":dot:"
)

func GetMessageLevelByStatus(status string) string {
	var color = map[string]string{
		"новый":                   statusCritical,
		"в работе (назначен)":     statusCritical,
		"в работе (запланирован)": statusWarning,
		"ожидающий":               statusWarning,
		"ожидающие":               statusWarning,
		"оценка":                  statusWarning,
		"принята":                 statusWarning,
		"тестирование":            statusWarning,
		"уточнение":               statusWarning,
		"рассмотрение":            statusWarning,
		"решен":                   statusSuccess,
		"применено":               statusSuccess,
		"закрыт":                  statusInfo,
		"закрыта":                 statusInfo,
	}

	if c, found := color[status]; found {
		return c
	}

	return statusInfo
}

func GetIconByStatus(status string) string {
	var ico = map[string]string{
		"решен":     iconComplate,
		"применено": iconComplate,
		"закрыт":    iconComplate,
		"закрыта":   iconComplate,
	}

	if c, found := ico[status]; found {
		return c
	}

	return iconWorking
}
