package jevapi

import (
	"fmt"
	"math"
)

const (
	answerNoul  = "noul"
	answerScore = "score"
)

// Question is one named question in a System One request.
type Question struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions,omitempty"`
	Criteria     any    `json:"criteria,omitempty"`
}

// NoulCriteria says what counts as a yes or a no answer.
type NoulCriteria struct {
	True  string `json:"true"`
	False string `json:"false"`
}

// NoulQuestion builds a yes/no question answered with the probability of yes.
func NoulQuestion(instructions, yes, no string) Question {
	return Question{Type: answerNoul, Instructions: instructions, Criteria: NoulCriteria{True: yes, False: no}}
}

// ScoreQuestion builds a rating question. The position of each level in
// levels is its score, starting at zero.
func ScoreQuestion(instructions string, levels ...string) Question {
	return Question{Type: answerScore, Instructions: instructions, Criteria: levels}
}

// Request is the body of POST /v1/systemone.
type Request struct {
	Model     string              `json:"model"`
	State     any                 `json:"state"`
	Questions map[string]Question `json:"questions"`
}

// Usage reports billed tokens.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Answer is one answer, keyed by question name in Response.Answers.
type Answer struct {
	Type       string   `json:"type"`
	NoulValue  *float64 `json:"noul"`
	Score      *float64 `json:"score"`
	Confidence *float64 `json:"confidence"`
}

// HasNoul reports whether the answer is a yes/no answer with a value.
func (a Answer) HasNoul() bool { return a.Type == answerNoul && a.NoulValue != nil }

// HasScore reports whether the answer is a rating with a value.
func (a Answer) HasScore() bool { return a.Type == answerScore && a.Score != nil }

// Noul returns the probability of yes, or zero when the answer has none.
func (a Answer) Noul() float64 {
	if a.NoulValue == nil {
		return 0
	}
	return *a.NoulValue
}

// ScoreValue returns the expected rating, or zero when the answer has none.
func (a Answer) ScoreValue() float64 {
	if a.Score == nil {
		return 0
	}
	return *a.Score
}

// Response is a decoded 200 reply.
type Response struct {
	Model     string            `json:"model"`
	Answers   map[string]Answer `json:"answers"`
	Usage     Usage             `json:"usage"`
	RequestID string            `json:"-"`
}

// Model is one entry of GET /v1/models.
type Model struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ReleaseDate string `json:"release_date"`
}

func (r *Response) validate() error {
	if len(r.Answers) == 0 {
		return fmt.Errorf("%w: no answers", ErrMalformedResponse)
	}
	for name, a := range r.Answers {
		var v *float64
		switch a.Type {
		case answerNoul:
			v = a.NoulValue
		case answerScore:
			v = a.Score
		default:
			return fmt.Errorf("%w: answer %q has unsupported type %q", ErrMalformedResponse, name, a.Type)
		}
		if v == nil || math.IsNaN(*v) || math.IsInf(*v, 0) {
			return fmt.Errorf("%w: answer %q has no usable value", ErrMalformedResponse, name)
		}
	}
	return nil
}
