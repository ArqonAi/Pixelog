package qr

import (
	"path/filepath"
	"sync"
	"testing"
)

// Two conversions at once must not share frame files: they used to write
// frame_00000.png … into one directory and overwrite each other.
func TestGenerateFramesUsesItsOwnDirectory(t *testing.T) {
	g, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dirs := make([]string, 2)
	var wg sync.WaitGroup
	for i := range dirs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			paths, err := g.GenerateFrames([]Chunk{{Index: 0, Total: 1, Data: "doc"}})
			if err != nil {
				t.Error(err)
				return
			}
			dirs[i] = filepath.Dir(paths[0])
			if filepath.Base(paths[0]) != "frame_00000.png" {
				t.Errorf("frame name %s (ffmpeg reads frame_%%05d.png)", paths[0])
			}
		}(i)
	}
	wg.Wait()
	if dirs[0] == "" || dirs[0] == dirs[1] {
		t.Fatalf("conversions shared a frame directory: %v", dirs)
	}
}
