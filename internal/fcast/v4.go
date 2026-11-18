package fcast

import (
	"time"
)

// The types of v4's Message union, as its schema orders them.
const (
	msgLoad                 = 1
	msgProgressChanged      = 2
	msgVolumeChanged        = 3
	msgPlaybackStateChanged = 4
	msgSpeedChanged         = 5
	msgSenderIntroduction   = 6
	msgReceiverIntroduction = 7
	msgStopPlayback         = 8
	msgError                = 23
)

// The values of v4's PlaybackState.
const (
	stateIdle      = 0
	stateBuffering = 1
	statePlaying   = 2
	statePaused    = 3
	stateEnded     = 4
)

// The kinds of v4's errors a load fails of.
const (
	errorResourceNotFound  = 1
	errorUnsupportedFormat = 5
)

// The types of the MediaSource and Metadata unions.
const (
	sourceSingle  = 1
	sourceQueue   = 2
	metadataAudio = 2
)

// v4Packet is a Packet of the Message msg of type typ, as a Flatbuf
// packet's body.
func v4Packet(typ uint8, msg *table) []byte {
	return (&table{}).union(0, typ, msg).finish()
}

// v4Introduction is the SenderIntroduction of MixMusic.
func v4Introduction(name, version string) []byte {
	info := (&table{}).str(0, name).str(1, name).str(2, version)
	return v4Packet(msgSenderIntroduction, (&table{}).table(0, info))
}

// v4Load loads m, alone: a MediaItem with its metadata.
func v4Load(m media) []byte {
	item := (&table{}).str(0, m.container).str(1, m.url).time(2, m.start)
	if m.speed != 0 && m.speed != 1 {
		item.f32(4, float32(m.speed))
	}
	item.str(6, m.title).str(7, m.image)
	if m.artist != "" || m.album != "" {
		item.union(8, metadataAudio, (&table{}).str(0, m.artist).str(1, m.album))
	}
	return v4Packet(msgLoad, (&table{}).union(0, sourceSingle, item))
}

func v4State(state uint8) []byte {
	return v4Packet(msgPlaybackStateChanged, (&table{}).u8(0, state))
}

func v4Seek(t, duration time.Duration) []byte {
	return v4Packet(msgProgressChanged, (&table{}).time(0, t).time(1, duration))
}

func v4Volume(v float64) []byte {
	return v4Packet(msgVolumeChanged, (&table{}).f32(0, float32(v)))
}

func v4Speed(v float64) []byte {
	return v4Packet(msgSpeedChanged, (&table{}).f32(0, float32(v)))
}

func v4Stop() []byte { return v4Packet(msgStopPlayback, &table{}) }

// v4Message is a Message a receiver sent, as far as MixMusic reads it.
type v4Message struct {
	typ uint8
	// state is a PlaybackStateChanged's.
	state uint8
	// position and duration are a ProgressChanged's.
	position, duration time.Duration
	hasDuration        bool
	// volume and speed are a VolumeChanged's and a SpeedChanged's.
	volume, speed float64
	// url is the address of the media of a Load.
	url string
	// errorKind is an Error's kind.
	errorKind uint8
	// containers and audio are the media capabilities of a
	// ReceiverIntroduction, as tokens, as "webm" and "opus".
	containers, audio []string
	app               string
}

// readV4 reads a Packet.
func readV4(body []byte) (m v4Message, err error) {
	defer func() {
		// The reader checks its bounds; this keeps a slip of it from
		// taking MixMusic down over a receiver's packet.
		if recover() != nil {
			m, err = v4Message{}, errFlat
		}
	}()
	root, err := rootOf(body)
	if err != nil {
		return v4Message{}, err
	}
	typ, msg := root.union(0)
	m.typ = typ
	switch typ {
	case msgPlaybackStateChanged:
		m.state, _ = msg.u8(0)
	case msgProgressChanged:
		m.position, _ = msg.time(0)
		m.duration, m.hasDuration = msg.time(1)
	case msgVolumeChanged:
		v, _ := msg.f32(0)
		m.volume = float64(v)
	case msgSpeedChanged:
		v, _ := msg.f32(0)
		m.speed = float64(v)
	case msgLoad:
		switch kind, src := msg.union(0); kind {
		case sourceSingle:
			m.url = src.str(1)
		case sourceQueue:
			if items := src.tables(0); len(items) > 0 {
				if item, ok := items[0].table(0); ok {
					m.url = item.str(1)
				}
			}
		}
	case msgError:
		m.errorKind, _ = msg.u8(0)
	case msgReceiverIntroduction:
		if info, ok := msg.table(0); ok {
			m.app = info.str(1)
		}
		if caps, ok := msg.table(1); ok {
			if media, ok := caps.table(0); ok {
				m.containers, m.audio = media.strs(1), media.strs(3)
			}
		}
	}
	return m, nil
}
