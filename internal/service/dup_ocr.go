package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/erikharutyunyan/go-file-manager/internal/domain"
	"github.com/erikharutyunyan/go-file-manager/internal/imghash"
	"github.com/erikharutyunyan/go-file-manager/internal/ocr"
)

func (s *FileService) runOCRPass(
	ctx context.Context,
	jobID, proto string,
	files []dupFile,
	collected *[]domain.DuplicateGroup,
	groups, skipN *int,
	doneImages, totalImages *int64,
	minPct int,
	tick func(path string, force bool),
) error {
	exact := exactPathSet(*collected)
	var cands []dupFile
	for _, f := range files {
		if _, ok := exact[f.Path]; ok {
			continue
		}
		if imghash.IsHEICName(f.Name) {
			*skipN++
			s.emit("dup:error", domain.DupErrorPayload{
				JobID: jobID, Path: f.Path, Message: "heic decode not supported", Fatal: false,
			})
			continue
		}
		if !imghash.IsVisualName(f.Name) {
			continue
		}
		if f.Size > imghash.MaxBytes {
			*skipN++
			s.emit("dup:error", domain.DupErrorPayload{
				JobID: jobID, Path: f.Path, Message: "too large for OCR", Fatal: false,
			})
			continue
		}
		cands = append(cands, f)
	}
	*totalImages = int64(len(cands))
	*doneImages = 0
	tick("", true)
	if len(cands) < 2 {
		return nil
	}

	var items []ocr.Item
	for _, f := range cands {
		if err := ctx.Err(); err != nil {
			return err
		}
		text, herr := s.ocrPath(ctx, jobID, f.Path)
		if herr != nil {
			if ctx.Err() != nil || isDupFatalErr(herr) {
				return herr
			}
			*skipN++
			s.emit("dup:error", domain.DupErrorPayload{
				JobID: jobID, Path: f.Path, Message: herr.Error(), Fatal: false,
			})
			*doneImages++
			tick(f.Path, false)
			continue
		}
		toks := ocr.Tokens(text)
		if len(toks) == 0 {
			*doneImages++
			tick(f.Path, false)
			continue
		}
		items = append(items, ocr.Item{Path: f.Path, Tokens: toks, Text: text})
		*doneImages++
		tick(f.Path, false)
	}

	byPath := map[string]dupFile{}
	for _, f := range cands {
		byPath[f.Path] = f
	}
	for _, cl := range ocr.Cluster(items, minPct) {
		g := ocrGroup(cl, byPath, proto)
		if len(g.Files) < 2 {
			continue
		}
		*groups++
		*collected = append(*collected, g)
	}
	return nil
}

func ocrGroup(cl ocr.Group, byPath map[string]dupFile, proto string) domain.DuplicateGroup {
	files := make([]dupFile, 0, len(cl.Items))
	for _, it := range cl.Items {
		if f, ok := byPath[it.Path]; ok {
			files = append(files, f)
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	paths := make([]string, len(files))
	g := domain.DuplicateGroup{Kind: "ocr", Similarity: cl.Similarity, Snippet: cl.Snippet}
	for i, m := range files {
		paths[i] = m.Path
		g.Files = append(g.Files, domain.DuplicateFile{
			Path: m.Path, Name: m.Name, Size: m.Size, ModTime: m.ModTime, Protocol: proto,
		})
	}
	if len(files) > 0 {
		g.Size = files[0].Size
	}
	sum := sha256.Sum256([]byte(strings.Join(paths, "\n")))
	g.Hash = "ocr:" + hex.EncodeToString(sum[:])
	return g
}
