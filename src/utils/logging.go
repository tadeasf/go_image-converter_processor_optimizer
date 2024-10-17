package utils

import (
	"fmt"
	"log"
)

func LogError(err error, verbose bool) {
	log.Printf("ERROR: %v", err)
	if verbose {
		fmt.Printf("ERROR: %v\n", err)
	}
}
