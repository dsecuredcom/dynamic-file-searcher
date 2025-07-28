package utils

import (
	"bufio"
	"log"
	"math/rand"
	"os"
)

func ReadLines(filename string) []string {
	var scanner *bufio.Scanner
	
	if filename == "-" {
		scanner = bufio.NewScanner(os.Stdin)
	} else {
		file, err := os.Open(filename)
		if err != nil {
			log.Fatalf("Error opening file %s: %v\n", filename, err)
		}
		defer file.Close()
		scanner = bufio.NewScanner(file)
	}

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading %s: %v\n", filename, err)
	}

	return lines
}

func ShuffleStrings(slice []string) []string {
	for i := len(slice) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		slice[i], slice[j] = slice[j], slice[i]
	}
	return slice
}
