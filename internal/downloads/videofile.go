package downloads

// VideoFile returns the file of a video's audio, downloaded for whichever
// track: an absolute path, which the engine plays the video from.
func (s *Snapshot) VideoFile(video string) (string, bool) {
	if s == nil {
		return "", false
	}
	p, ok := s.files[video]
	return p, ok
}
