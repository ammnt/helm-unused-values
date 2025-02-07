package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

func main() {
	// Определение флага для пути к values.yaml
	valuesPath := flag.String("values", "values.yaml", "Path to the values.yaml file")
	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Println("Usage: go run main.go -values <path-to-values.yaml> <path-to-helm-chart>")
		return
	}

	chartPath := flag.Args()[0]
	templatesPath := filepath.Join(chartPath, "templates")

	// Чтение values.yaml
	valuesData, err := ioutil.ReadFile(*valuesPath)
	if err != nil {
		fmt.Printf("Failed to read values.yaml: %v\n", err)
		return
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(valuesData, &values); err != nil {
		fmt.Printf("Failed to unmarshal values.yaml: %v\n", err)
		return
	}

	// Сбор всех используемых значений из шаблонов
	usedValues := make(map[string]bool)
	err = filepath.Walk(templatesPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		data, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		re := regexp.MustCompile(`\.Values\.([a-zA-Z0-9_.]+)`)
		matches := re.FindAllStringSubmatch(string(data), -1)
		for _, match := range matches {
			usedValues[match[1]] = true
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Failed to walk templates directory: %v\n", err)
		return
	}

	// Поиск неиспользуемых значений
	unusedValues := findUnusedValues(values, usedValues, "")
	if len(unusedValues) > 0 {
		fmt.Println("Unused values:")
		for _, value := range unusedValues {
			fmt.Println(value)
		}
	} else {
		fmt.Println("No unused values found.")
	}
}

func findUnusedValues(values map[string]interface{}, usedValues map[string]bool, prefix string) []string {
	var unused []string

	for key, value := range values {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch v := value.(type) {
		case map[string]interface{}:
			unused = append(unused, findUnusedValues(v, usedValues, fullKey)...)
		default:
			if !usedValues[fullKey] {
				unused = append(unused, fullKey)
			}
		}
	}

	return unused
}