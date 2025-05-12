package audio

import (
	"io/fs"
	"log"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

func LoadWAV(fsys fs.FS, path string) *audio.Player {
	f, err := fsys.Open(path)
	if err != nil {
		log.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	d, err := wav.Decode(audio.CurrentContext(), f)
	if err != nil {
		log.Fatalf("decode %s: %v", path, err)
	}

	p, err := audio.NewPlayer(audio.CurrentContext(), d)
	if err != nil {
		log.Fatalf("new player %s: %v", path, err)
	}
	return p
}
