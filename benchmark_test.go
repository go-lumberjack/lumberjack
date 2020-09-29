package lumberjack

import (
	"log"
	"os"
	"testing"
)

func BenchmarkNotBufferSize(b *testing.B) {
	filename := "testNotBuffer.log"
	defer os.Remove(filename) // nolint

	l, _ := New(
		WithFileName(filename),
	)
	log.SetOutput(l)
	for i := 0; i < b.N; i++ {
		log.Println("booo!")
	}
}

func BenchmarkWithBufferSize(b *testing.B) {
	filename := "testWithBuffer.log"
	defer os.Remove(filename) // nolint

	l, _ := New(
		WithFileName(filename),
		WithBufferSize(1*KB),
	)
	log.SetOutput(l)
	for i := 0; i < b.N; i++ {
		log.Println("booo!")
	}
}
