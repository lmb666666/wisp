package service

import (
	"testing"

	"github.com/lmb666666/wisp/internal/repo"
)

func TestExtractTagsIgnoresCode(t *testing.T) {
	src := "```c\n#include <stdio.h>\n#define X 1\n```\n\n正文 #真实标签 和 `#inline`"
	tags := ExtractTags(src)
	for _, tag := range tags {
		if tag == "include" || tag == "define" || tag == "inline" {
			t.Errorf("code content extracted as tag: %v", tags)
		}
	}
	if len(tags) != 1 || tags[0] != "真实标签" {
		t.Fatalf("tags = %v, want [真实标签]", tags)
	}
}

func TestRemoteThumbnailFallsBackToPersistedURL(t *testing.T) {
	ps := &PostService{}
	dto := ps.attachmentDTO(repo.Attachment{
		Kind: "image", Storage: "remote", StorageKey: "k",
		RemoteUrl: "https://cdn.example/x.jpg", ThumbPath: "x.jpg.thumb.jpg",
	})
	if dto.ThumbURL != "https://cdn.example/x.jpg" {
		t.Fatalf("legacy remote thumb = %q", dto.ThumbURL)
	}

	dto = ps.attachmentDTO(repo.Attachment{
		Kind: "image", Storage: "s3", StorageKey: "k",
		RemoteUrl: "https://cdn.example/x.jpg", ThumbPath: "x.jpg.thumb.jpg",
		ThumbRemoteUrl: "https://cdn.example/x.jpg.thumb.jpg",
	})
	if dto.ThumbURL != "https://cdn.example/x.jpg.thumb.jpg" {
		t.Fatalf("s3 thumb = %q", dto.ThumbURL)
	}
}
