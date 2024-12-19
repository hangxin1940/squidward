package audio

import (
	"bytes"
	"cmp"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"squidward/lib"
	"strconv"
	"strings"
	"testing"
)

func _loadFrames(aid string) Audio {
	rpath := filepath.Join(lib.RuntimeDir(), "../../", "tmp", fmt.Sprintf("audioframe_%s", aid))
	entries, err := os.ReadDir(rpath)
	if err != nil {
		panic(err)
	}

	files := []string{}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fns := strings.Split(entry.Name(), "_")
		if len(fns) != 2 {
			continue
		}

		_, erra := strconv.Atoi(fns[0])
		if erra != nil {
			continue
		}

		files = append(files, entry.Name())
	}

	files = slices.SortedFunc(slices.Values(files), func(m1, m2 string) int {
		fns1 := strings.Split(m1, "_")
		fns2 := strings.Split(m2, "_")

		i1, _ := strconv.Atoi(fns1[0])
		i2, _ := strconv.Atoi(fns2[0])
		return cmp.Compare(i1, i2)
	})

	af := Audio{
		Mime:   "audio/L16;rate=8000",
		Frames: []Frame{},
	}
	for i, f := range files {
		bs, errf := os.ReadFile(path.Join(rpath, f))
		if errf != nil {
			panic(err)
		}
		af.AddFrame(i, bs)
	}

	return af
}

func TestAudio_parseFrames(t *testing.T) {
	id := "ws"
	af := _loadFrames(id)

	audiobytes := af.ToAudioBytesReader()
	buf := new(bytes.Buffer)
	buf.ReadFrom(audiobytes)
	audiofile := filepath.Join(lib.RuntimeDir(), "../../", "tmp", fmt.Sprintf("audioframe_%s", id), "audio.wav")
	err := os.WriteFile(audiofile, buf.Bytes(), 0644)
	if err != nil {
		t.Fatal(err)
	}
}
