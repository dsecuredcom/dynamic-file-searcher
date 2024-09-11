package utils

import (
	"bufio"
	"log"
	"os"
)

// ReadLines reads a file and returns its contents as a slice of strings
func ReadLines(filename string) []string {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Error opening file %s: %v\n", filename, err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading file %s: %v\n", filename, err)
	}

	return lines
}
