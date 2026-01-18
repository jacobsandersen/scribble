package media

import "bytes"

type testFile struct{ *bytes.Reader }

func (f testFile) Close() error { return nil }
