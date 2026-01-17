package media

import "testing"

func TestS3MediaStore_objectURL(t *testing.T) {
	store := &S3MediaStore{bucket: "bucket", endpointHost: "s3.example.com", secure: true}

	if got := store.objectURL("path/to/key"); got != "https://bucket.s3.example.com/path/to/key" {
		t.Fatalf("unexpected virtual-host url: %s", got)
	}

	store.forcePathStyle = true
	if got := store.objectURL("path/to/key"); got != "https://s3.example.com/bucket/path/to/key" {
		t.Fatalf("unexpected path-style url: %s", got)
	}

	store.publicBase = "https://cdn.example.com/media"
	if got := store.objectURL("path/to/key"); got != "https://cdn.example.com/media/path/to/key" {
		t.Fatalf("unexpected public url: %s", got)
	}
}

func TestS3MediaStore_keyFromURL(t *testing.T) {
	store := &S3MediaStore{bucket: "bucket"}

	cases := []struct {
		name   string
		url    string
		expect string
	}{
		{"virtual host", "https://bucket.s3.example.com/path/to/key", "path/to/key"},
		{"path style", "https://s3.example.com/bucket/path/to/key", "path/to/key"},
		{"plain path", "https://example.com/path/to/key", "path/to/key"},
	}

	for _, tc := range cases {
		got, err := store.keyFromURL(tc.url)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if got != tc.expect {
			t.Fatalf("%s: expected %s, got %s", tc.name, tc.expect, got)
		}
	}
}
