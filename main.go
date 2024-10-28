package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// TreeNode представляет узел дерева значений
type TreeNode struct {
	Children map[string]*TreeNode
}

func NewTreeNode() *TreeNode {
	return &TreeNode{Children: make(map[string]*TreeNode)}
}

func (n *TreeNode) AddPath(path []string) {
	if len(path) == 0 {
		return
	}
	child, exists := n.Children[path[0]]
	if !exists {
		child = NewTreeNode()
		n.Children[path[0]] = child
	}
	child.AddPath(path[1:])
}

func (n *TreeNode) IsUsed(path []string) bool {
	if len(path) == 0 {
		return true
	}
	if child, exists := n.Children[path[0]]; exists {
		return child.IsUsed(path[1:])
	}
	return false
}

// Build tree from YAML values
func buildTree(values map[string]interface{}, node *TreeNode, prefix string) {
	for key, val := range values {
		fullKey := strings.TrimPrefix(prefix+"."+key, ".")
		if isEmpty(val) {
			continue
		}
		node.AddPath(strings.Split(fullKey, "."))
		if subMap, ok := val.(map[string]interface{}); ok {
			buildTree(subMap, node, fullKey)
		}
	}
}

// Check if the value is empty
func isEmpty(val interface{}) bool {
	switch v := val.(type) {
	case string:
		return v == ""
	case bool:
		return !v
	case []interface{}:
		return len(v) == 0
	case map[string]interface{}:
		return len(v) == 0
	default:
		return false
	}
}

// Read values from YAML file and build tree
func readValuesTree(filePath string) (*TreeNode, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read values file: %w", err)
	}
	var valuesMap map[string]interface{}
	if err := yaml.Unmarshal(data, &valuesMap); err != nil {
		return nil, fmt.Errorf("failed to parse values file: %w", err)
	}
	root := NewTreeNode()
	buildTree(valuesMap, root, "")
	return root, nil
}

// Extract .Values paths from templates
func extractValuesPathsFromTemplates(templateContents []string) []string {
	var paths []string
	regex := regexp.MustCompile(`{{\s*\.Values\.([\w\.]+)\s*}}`)
	for _, content := range templateContents {
		matches := regex.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				paths = append(paths, match[1])
			}
		}
	}
	return paths
}

// Build used values tree from extracted paths
func buildUsedValuesTree(usedValues []string) *TreeNode {
	root := NewTreeNode()
	for _, value := range usedValues {
		root.AddPath(strings.Split(value, "."))
	}
	return root
}

// Find unused values by comparing trees
func findUnusedValues(valuesTree, usedValuesTree *TreeNode, prefix string) []string {
	var unusedValues []string
	for key, childNode := range valuesTree.Children {
		fullKey := prefix + "." + key

		// Check if the current node exists in the used values tree
		if _, exists := usedValuesTree.Children[key]; !exists {
			// If the current node does not exist, check if it has any children
			if len(childNode.Children) > 0 {
				// If it has children, skip this node
				continue
			}
			// If no children, add to unused values
			unusedValues = append(unusedValues, fullKey)
		} else {
			// If the node exists, recursively check its children
			unusedValues = append(unusedValues, findUnusedValues(childNode, usedValuesTree.Children[key], fullKey)...)
		}
	}
	return unusedValues
}

func main() {
	helmChartPath := flag.String("chart", "", "Path to the Helm chart directory")
	valuesFilePath := flag.String("values", "values.yaml", "Path to the values.yaml file")
	devValuesFilePath := flag.String("dev-values", "", "Path to the values-dev.yaml file (optional)")
	flag.Parse()

	if *helmChartPath == "" {
		fmt.Println("Usage: go run main.go -chart=<helm-chart-path> [-values=<values.yaml>] [-dev-values=<values-dev.yaml>]")
		os.Exit(1)
	}

	templatesDirPath := filepath.Join(*helmChartPath, "templates")
	templateContents := readTemplates(templatesDirPath)

	usedValues := buildUsedValuesTree(extractValuesPathsFromTemplates(templateContents))
	valuesTree, err := readValuesTree(*valuesFilePath)
	if err != nil {
		fmt.Printf("error while reading root values: %v", err)
		os.Exit(1)
	}

	if *devValuesFilePath != "" {
		devValuesTree, err := readValuesTree(*devValuesFilePath)
		if err != nil {
			fmt.Printf("error while reading dev values: %v", err)
			os.Exit(1)
		}
		mergeTrees(valuesTree, devValuesTree)
	}

	unusedValues := findUnusedValues(valuesTree, usedValues, ".Values")
	fmt.Println("Unused values in values.yaml:")
	for _, value := range unusedValues {
		fmt.Println(value)
	}
}

// Read all template files from the specified directory
func readTemplates(dirPath string) []string {
	var templateContents []string
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		fmt.Printf("error while reading templates: %v", err)
		os.Exit(1)
	}
	for _, file := range files {
		if !file.IsDir() {
			content, err := ioutil.ReadFile(filepath.Join(dirPath, file.Name()))
			if err != nil {
				fmt.Printf("error reading template file %s: %v\n", file.Name(), err)
				os.Exit(1)
			}
			templateContents = append(templateContents, string(content))
		}
	}
	return templateContents
}

// Merge main and dev values trees
func mergeTrees(mainTree, devTree *TreeNode) {
	for key, devChild := range devTree.Children {
		if mainChild, exists := mainTree.Children[key]; exists {
			mergeTrees(mainChild, devChild)
		} else {
			mainTree.Children[key] = devChild
		}
	}
}
