-- The records an erasure or anonymize cleared from the AI-activity rail.
--
-- The rail is a projection: a task that was running during the erasure, or an
-- event relayed after it, carries the old name again. The projection reads this
-- list on every write and stores no label, summary or reason for a record on
-- it, so the clearing holds.
--
-- It holds ids only, never a name.
SET LOCAL lock_timeout = '3s';

CREATE TABLE ai_task_run_forgotten_subject (
    subject_type text NOT NULL,
    subject_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ai_task_run_forgotten_subject_pkey PRIMARY KEY (subject_type, subject_id)
);
