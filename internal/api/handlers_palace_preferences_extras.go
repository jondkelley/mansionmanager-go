package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"net/http"

	"palace-manager/internal/instance"
)

const ratbotMaxFileSize = 2 << 20

type triviaQuestionDTO struct {
	Question string   `json:"question"`
	Options  []string `json:"options"`
	Correct  string   `json:"correct"`
}

func normalizeRatbotFileName(name string) (string, error) {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "" || base == "." || base == ".." {
		return "", fmt.Errorf("invalid file name")
	}
	if strings.Contains(base, "/") || strings.Contains(base, `\`) || strings.Contains(base, "..") {
		return "", fmt.Errorf("invalid file name")
	}
	return base, nil
}

func (s *Server) palaceRatbotDir(palaceName string) (string, error) {
	dd, err := s.palaceDataDir(palaceName)
	if err != nil {
		return "", err
	}
	return filepath.Join(dd, "ratbot"), nil
}

func (s *Server) handlePalaceMiscGet(w http.ResponseWriter, r *http.Request, palaceName string) {
	if !canAccessPalace(r.Context(), palaceName) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("palace %q not found", palaceName))
		return
	}
	v, err := instance.ReadUnitVerbosity(palaceName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"verbosity": v,
		"range":     []int{1, 5},
	})
}

func (s *Server) handlePalaceMiscSave(w http.ResponseWriter, r *http.Request, palaceName string) {
	if !canAccessPalace(r.Context(), palaceName) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("palace %q not found", palaceName))
		return
	}
	var req struct {
		Verbosity int `json:"verbosity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Verbosity < 1 || req.Verbosity > 5 {
		writeError(w, http.StatusBadRequest, "verbosity must be between 1 and 5")
		return
	}
	if err := instance.PatchUnitVerbosity(palaceName, req.Verbosity); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.instances.Restart(palaceName); err != nil {
		writeError(w, http.StatusInternalServerError, "saved verbosity but restart failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "verbosity": req.Verbosity, "restarted": true})
}

func (s *Server) handlePalaceRatbotFilesList(w http.ResponseWriter, r *http.Request, palaceName string) {
	if !canAccessPalace(r.Context(), palaceName) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("palace %q not found", palaceName))
		return
	}
	dir, err := s.palaceRatbotDir(palaceName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, map[string]any{"dir": dir, "files": []string{}})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)
	writeJSON(w, http.StatusOK, map[string]any{"dir": dir, "files": files})
}

func parseTriviaLine(line string) (triviaQuestionDTO, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return triviaQuestionDTO{}, false
	}
	if len(line) < 3 || !strings.EqualFold(line[:2], "Q.") {
		return triviaQuestionDTO{}, false
	}
	rest := line[2:]
	type marker struct {
		pos     int
		letter  byte
		correct bool
	}
	var markers []marker
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		var letter byte
		var corr bool
		if c == '*' && i+2 < len(rest) && rest[i+1] >= 'A' && rest[i+1] <= 'D' && rest[i+2] == '.' {
			letter = rest[i+1]
			corr = true
		} else if c >= 'A' && c <= 'D' && i+1 < len(rest) && rest[i+1] == '.' {
			letter = c
		} else {
			continue
		}
		if i > 0 && !unicode.IsSpace(rune(rest[i-1])) {
			continue
		}
		markers = append(markers, marker{pos: i, letter: letter, correct: corr})
	}
	if len(markers) < 2 {
		return triviaQuestionDTO{}, false
	}
	qtext := strings.TrimSpace(rest[:markers[0].pos])
	if qtext == "" {
		return triviaQuestionDTO{}, false
	}
	options := make([]string, 4)
	correct := ""
	for i, m := range markers {
		skip := 2
		if m.correct {
			skip = 3
		}
		start := m.pos + skip
		end := len(rest)
		if i+1 < len(markers) {
			end = markers[i+1].pos
		}
		if start > end {
			return triviaQuestionDTO{}, false
		}
		idx := int(m.letter - 'A')
		if idx < 0 || idx > 3 {
			continue
		}
		options[idx] = strings.TrimSpace(rest[start:end])
		if m.correct {
			correct = string(m.letter)
		}
	}
	if correct == "" {
		return triviaQuestionDTO{}, false
	}
	for _, opt := range options {
		if strings.TrimSpace(opt) == "" {
			return triviaQuestionDTO{}, false
		}
	}
	return triviaQuestionDTO{Question: qtext, Options: options, Correct: correct}, true
}

func parseTriviaQuestions(content string) ([]triviaQuestionDTO, int) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	out := make([]triviaQuestionDTO, 0, len(lines))
	bad := 0
	for _, line := range lines {
		q, ok := parseTriviaLine(line)
		if ok {
			out = append(out, q)
			continue
		}
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			bad++
		}
	}
	return out, bad
}

func (s *Server) handlePalaceRatbotFileGet(w http.ResponseWriter, r *http.Request, palaceName string) {
	if !canAccessPalace(r.Context(), palaceName) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("palace %q not found", palaceName))
		return
	}
	name, err := normalizeRatbotFileName(r.URL.Query().Get("name"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	dir, err := s.palaceRatbotDir(palaceName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	full := filepath.Join(dir, name)
	b, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(b) > ratbotMaxFileSize {
		writeError(w, http.StatusRequestEntityTooLarge, "ratbot file too large")
		return
	}
	questions, bad := parseTriviaQuestions(string(b))
	writeJSON(w, http.StatusOK, map[string]any{
		"name":             name,
		"questionCount":    len(questions),
		"invalidLineCount": bad,
		"questions":        questions,
	})
}

func renderTriviaQuestions(questions []triviaQuestionDTO) string {
	var b strings.Builder
	b.WriteString("# Ratbot trivia file\n")
	b.WriteString("# Format: Q. question A. one *B. correct C. three D. four\n")
	for _, q := range questions {
		correctIdx := int(strings.ToUpper(strings.TrimSpace(q.Correct))[0] - 'A')
		b.WriteString("Q. ")
		b.WriteString(strings.TrimSpace(q.Question))
		for i := 0; i < 4; i++ {
			b.WriteString(" ")
			letter := string(rune('A' + i))
			if i == correctIdx {
				b.WriteString("*")
			}
			b.WriteString(letter)
			b.WriteString(". ")
			b.WriteString(strings.TrimSpace(q.Options[i]))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func validateTriviaQuestions(in []triviaQuestionDTO) ([]triviaQuestionDTO, error) {
	out := make([]triviaQuestionDTO, 0, len(in))
	for i, q := range in {
		q.Question = strings.TrimSpace(q.Question)
		if q.Question == "" {
			return nil, fmt.Errorf("question %d is empty", i+1)
		}
		if len(q.Options) != 4 {
			return nil, fmt.Errorf("question %d must have 4 answers", i+1)
		}
		opts := make([]string, 4)
		for j := 0; j < 4; j++ {
			opts[j] = strings.TrimSpace(q.Options[j])
			if opts[j] == "" {
				return nil, fmt.Errorf("question %d has an empty answer", i+1)
			}
		}
		correct := strings.ToUpper(strings.TrimSpace(q.Correct))
		if correct != "A" && correct != "B" && correct != "C" && correct != "D" {
			return nil, fmt.Errorf("question %d has invalid correct answer", i+1)
		}
		out = append(out, triviaQuestionDTO{
			Question: q.Question,
			Options:  opts,
			Correct:  correct,
		})
	}
	return out, nil
}

func (s *Server) handlePalaceRatbotFileSave(w http.ResponseWriter, r *http.Request, palaceName string) {
	if !canAccessPalace(r.Context(), palaceName) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("palace %q not found", palaceName))
		return
	}
	var req struct {
		Name      string              `json:"name"`
		Questions []triviaQuestionDTO `json:"questions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	name, err := normalizeRatbotFileName(req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	questions, err := validateTriviaQuestions(req.Questions)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(questions) == 0 {
		writeError(w, http.StatusBadRequest, "add at least one question")
		return
	}
	content := renderTriviaQuestions(questions)
	if len(content) > ratbotMaxFileSize {
		writeError(w, http.StatusRequestEntityTooLarge, "ratbot file too large")
		return
	}
	dir, err := s.palaceRatbotDir(palaceName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	full := filepath.Join(dir, name)
	if err := writeFileAtomicAs(full, s.palaceLinuxUser(palaceName), strings.NewReader(content)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name, "questionCount": len(questions)})
}
