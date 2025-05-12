//Handles reading and decoding WAV files from the embedded assets/sfx folder and creating Ebiten audio players.

package audio

import (
	"io/fs"
	"log"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

// LoadWAV opens a WAV file from the given filesystem (e.g., an embedded FS),
// decodes it, and returns an *audio.Player ready to play that sound.
// It logs fatal errors if any step fails.
func LoadWAV(fsys fs.FS, path string) *audio.Player {
	// 1) Open the file handle for the WAV at the specified path.
	f, err := fsys.Open(path)
	if err != nil {
		// If the file can’t be opened, log the error and exit immediately.
		log.Fatalf("open %s: %v", path, err)
	}
	defer f.Close() // ensure we close the file once done

	// 2) Decode the WAV data into an audio stream using the current audio context.
	//    This reads the PCM data and prepares it for playback.
	d, err := wav.Decode(audio.CurrentContext(), f)
	if err != nil {
		log.Fatalf("decode %s: %v", path, err)
	}

	// 3) Create a new audio.Player from the decoded stream.
	//    The Player handles buffering and playback controls.
	p, err := audio.NewPlayer(audio.CurrentContext(), d)
	if err != nil {
		log.Fatalf("new player %s: %v", path, err)
	}

	// 4) Return the fully configured Player so the caller can play, rewind, and control volume.
	return p
}
