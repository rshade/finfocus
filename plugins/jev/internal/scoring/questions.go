package scoring

import (
	"fmt"

	"github.com/rshade/finfocus/plugins/jev/internal/jevapi"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

const (
	signalRisk        = "risk"
	signalFalsePos    = "false_positive"
	signalWorth       = "worth_acting"
	signalPriority    = "priority"
	signalEvidence    = "insufficient_evidence"
	signalDuplicate   = "duplicate"
	positionPrefixFmt = "For the item whose position is %d in the state list, and for no other item: "
)

// supportedSignals lists every signal the scorer can return, in the order it
// reports them.
func supportedSignals() []pbc.ScoreSignal {
	return []pbc.ScoreSignal{
		pbc.ScoreSignal_SCORE_SIGNAL_RISK,
		pbc.ScoreSignal_SCORE_SIGNAL_FALSE_POSITIVE,
		pbc.ScoreSignal_SCORE_SIGNAL_WORTH_ACTING,
		pbc.ScoreSignal_SCORE_SIGNAL_PRIORITY,
		pbc.ScoreSignal_SCORE_SIGNAL_INSUFFICIENT_EVIDENCE,
		pbc.ScoreSignal_SCORE_SIGNAL_DUPLICATE_GROUP,
	}
}

// perRecordSignal is a signal answered by one question per recommendation.
type perRecordSignal struct {
	signal pbc.ScoreSignal
	name   string
	build  func(position int) jevapi.Question
}

func perRecordSignals() []perRecordSignal {
	return []perRecordSignal{
		{pbc.ScoreSignal_SCORE_SIGNAL_RISK, signalRisk, riskQuestion},
		{pbc.ScoreSignal_SCORE_SIGNAL_FALSE_POSITIVE, signalFalsePos, falsePositiveQuestion},
		{pbc.ScoreSignal_SCORE_SIGNAL_WORTH_ACTING, signalWorth, worthActingQuestion},
		{pbc.ScoreSignal_SCORE_SIGNAL_PRIORITY, signalPriority, priorityQuestion},
		{pbc.ScoreSignal_SCORE_SIGNAL_INSUFFICIENT_EVIDENCE, signalEvidence, insufficientEvidenceQuestion},
	}
}

func positionPrefix(position int) string {
	return fmt.Sprintf(positionPrefixFmt, position)
}

func riskQuestion(position int) jevapi.Question {
	return jevapi.NoulQuestion(
		positionPrefix(position)+"would applying this recommendation as described carry a real risk of downtime, "+
			"data loss, performance regression, or locking in a financial commitment that is hard to undo, "+
			"judging by the resource's environment, tags and usage in the data?",
		"Applying it could plausibly hurt users, lose data, or lock in a commitment that is hard to undo",
		"Applying it is low risk and reversible given the evidence",
	)
}

func falsePositiveQuestion(position int) jevapi.Question {
	return jevapi.NoulQuestion(
		positionPrefix(position)+"is this recommendation probably inappropriate for this resource because the data "+
			"suggests it looks idle or oversized on purpose, for example a standby, seasonal or batch workload, "+
			"compliance retention, or deliberate headroom?",
		"The evidence points to a legitimate reason for the current state",
		"The resource really is wasteful as described",
	)
}

func worthActingQuestion(position int) jevapi.Question {
	return jevapi.NoulQuestion(
		positionPrefix(position)+"is this recommendation worth an engineer's time this week, weighing the monthly "+
			"saving against the risk and effort?",
		"Meaningful saving, low risk and modest effort",
		"Small saving, high risk or high effort, or not clearly correct",
	)
}

func priorityQuestion(position int) jevapi.Question {
	return jevapi.ScoreQuestion(
		positionPrefix(position)+"how much priority should this cost recommendation get, weighing saving, risk "+
			"and whether the evidence supports it?",
		"Ignore", "Low", "Medium", "High",
	)
}

func insufficientEvidenceQuestion(position int) jevapi.Question {
	return jevapi.NoulQuestion(
		positionPrefix(position)+"is the evidence in this record too thin or contradictory to tell whether the "+
			"recommendation is correct?",
		"The record is too thin or contradictory to judge",
		"The record holds enough evidence to judge",
	)
}

func duplicateQuestion(position int) jevapi.Question {
	return jevapi.NoulQuestion(
		positionPrefix(position)+"do recommendation a and recommendation b describe the same underlying change to "+
			"the same cloud resource, so that acting on one makes the other redundant?",
		"Same resource and same intended change, even if worded differently or from different tools",
		"Different resources, or a different change to the same resource",
	)
}

// questionName is the key of a question in the request: the signal, a colon
// and the item's position in the state list. The recommendation id is never
// used, so no plugin-supplied text shapes a key and no identifier leaves the
// host through one; answers are mapped back to ids by position.
func questionName(signal string, position int) string {
	return fmt.Sprintf("%s:#%d", signal, position)
}

func pairQuestionName(position int) string {
	return fmt.Sprintf("%s:%d", signalDuplicate, position)
}
