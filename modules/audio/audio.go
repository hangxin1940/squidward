package audio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strconv"
	"strings"
)

func NewAudio(mime string) *Audio {
	return &Audio{
		Mime:   mime,
		Frames: []Frame{},
	}
}

type Frame struct {
	Index int
	Data  []byte
}

type Audio struct {
	Mime   string
	Frames []Frame
}

func (a *Audio) AddFrame(index int, frame []byte) {
	a.Frames = append(a.Frames, Frame{
		Index: index,
		Data:  frame,
	})
}

func (a *Audio) AssembleFrames() []byte {
	audioBytes := new(bytes.Buffer)
	for _, frame := range a.Frames {
		audioBytes.Write(frame.Data)
	}
	return audioBytes.Bytes()
}

func (a *Audio) ToAudioBytesReader() io.Reader {
	// 添加音频头
	audioBytes := new(bytes.Buffer)
	bodyBytes := a.AssembleFrames()

	audioHeader, err := generateRIFFHeader(a.Mime, len(bodyBytes))
	if err != nil {
		return nil
	}
	audioBytes.Write(audioHeader)
	audioBytes.Write(bodyBytes)

	return audioBytes
}

type audioProperties struct {
	audioFormat   uint16
	sampleRate    uint32
	bitsPerSample uint16
}

func matchFormat(value string) uint16 {
	switch value {
	case "S16LE", "L16":
		return 16
	case "S8":
		return 8
	}
	return 0
}

func generateRIFFHeader(mime string, dataSize int) ([]byte, error) {
	// TODO 优化
	// Default properties
	props := audioProperties{audioFormat: 1} // Default to PCM

	parts := strings.Split(mime, ";")
	if len(parts) < 1 {
		return nil, errors.New("invalid mime type")
	}

	for _, part := range parts[1:] {
		keyValue := strings.Split(part, "=")
		if len(keyValue) != 2 {
			continue
		}
		key, value := keyValue[0], keyValue[1]
		switch key {
		case "rate":
			sampleRate, err := strconv.Atoi(value)
			if err != nil {
				return nil, errors.New("invalid sample rate")
			}
			props.sampleRate = uint32(sampleRate)
		case "format":
			props.bitsPerSample = matchFormat(value)
			props.bitsPerSample = 8
		}
	}

	if props.bitsPerSample == 0 {
		fts := strings.Split(parts[0], "/")
		if len(fts) < 1 {
			return nil, errors.New("invalid mime type")
		}
		props.bitsPerSample = matchFormat(fts[1])
	}

	if props.sampleRate == 0 || props.bitsPerSample == 0 {
		return nil, errors.New("unsupported mime type")
	}

	blockAlign := props.bitsPerSample / 8
	byteRate := props.sampleRate * uint32(blockAlign)

	header := &bytes.Buffer{}
	switch parts[0] {
	case "audio/L16", "audio/x-raw", "audio/basic", "audio/x-alaw-basic":
		header.Write([]byte("RIFF"))
		binary.Write(header, binary.LittleEndian, uint32(36+dataSize))
		header.Write([]byte("WAVE"))

		header.Write([]byte("fmt "))
		binary.Write(header, binary.LittleEndian, uint32(16))
		binary.Write(header, binary.LittleEndian, props.audioFormat)
		binary.Write(header, binary.LittleEndian, uint16(1))
		binary.Write(header, binary.LittleEndian, props.sampleRate)
		binary.Write(header, binary.LittleEndian, byteRate)
		binary.Write(header, binary.LittleEndian, blockAlign)
		binary.Write(header, binary.LittleEndian, props.bitsPerSample)

		header.Write([]byte("data"))
		binary.Write(header, binary.LittleEndian, uint32(dataSize))

	case "audio/ogg", "audio/opus":
		header.Write([]byte("OggS"))
		// OGG/Opus specific header fields can be added here

	case "audio/mp3":
		header.Write([]byte("ID3"))
		// MP3 specific header fields can be added here

	default:
		return nil, errors.New("unsupported audio format")
	}

	return header.Bytes(), nil
}

func CheckMimeValid(mime string) bool {
	if _, err := generateRIFFHeader(mime, 0); err == nil {
		return true
	}

	return false
}
