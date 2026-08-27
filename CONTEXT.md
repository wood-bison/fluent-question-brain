# Question Brain

Question Brain owns immutable interview-question content, reviewed knowledge
placement, and release-pinned coverage policy.

## Language

**QuestionCard**:
One stable question identity whose localized content changes only through an immutable revision.
_Avoid_: Lesson, task, learner step

**QuestionCapabilityBinding**:
A reviewed relation between one QuestionCard revision and one Capability, including its canonical semantic role.
_Avoid_: Topic inference, coverage target

**PrimaryQuestion**:
The learner-visible main question derived from a `primary` QuestionCapabilityBinding.
_Avoid_: Primary card copy, lesson

**SupportingPrompt**:
A learner-visible prerequisite, recall, contrast, follow-up, or supporting-evidence prompt derived from the corresponding QuestionCapabilityBinding role.
_Avoid_: Secondary question count, duplicated card

**CoverageTarget**:
A release-pinned minimum of PrimaryQuestions and SupportingPrompts required for one Path and Capability.
_Avoid_: Raw card quota, capability registry

**CoverageDisposition**:
A reviewed classification of a pinned card as `core`, `supplemental`, or `quarantined` for one coverage-target release.
_Avoid_: Capability-binding disposition, publication status
