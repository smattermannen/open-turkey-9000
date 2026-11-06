// try-challenge runs the unlock typing challenge on its own, for practice or
// testing. It touches no blocks, needs no root and changes nothing:
//
//	./try-challenge.sh          # default length
//	./try-challenge.sh 100      # 100 characters
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/smattermannen/kc9k/internal/lock"
)

func main() {
	length := lock.DefaultLockChars
	if len(os.Args) > 1 {
		n, err := strconv.Atoi(os.Args[1])
		if err != nil || lock.CheckLockChars(n) != nil {
			fmt.Fprintf(os.Stderr, "usage: ./try-challenge.sh [length]  (%d to %d, default %d)\n", lock.MinLockChars, lock.MaxLockChars, lock.DefaultLockChars)
			os.Exit(2)
		}
		length = n
	}

	passed, err := lock.RunChallenge(length)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !passed {
		fmt.Println("(Practice run: a real block would have stayed locked.)")
		os.Exit(1)
	}
	fmt.Println("(Practice run: a real block would now be unlocked.)")
}
