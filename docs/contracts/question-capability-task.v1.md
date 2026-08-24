# Question → Capability → Task contract v1

This is the Question Brain-owned side of the workspace contract. The complete
machine-readable schema and representative fixture are in:

- `../../docs/contracts/question-capability-task.v1.schema.json`
- `../../docs/contracts/question-capability-task.v1.fixture.json`

The paths above resolve in the `fluent-interview` workspace. A standalone
checkout must copy those immutable files into its release tooling and verify
their SHA-256 before publishing a cross-repository manifest.

Question Brain owns `QuestionCard`, `Capability`,
`CapabilityDomainBinding`, `QuestionCapabilityBinding`, and `ContentRelation`.
It never owns executable task source, hidden tests, Run records, or learner
Evidence. All bindings are revision-pinned and carry provenance; proposed
facts are not exposed by a released learner projection.

