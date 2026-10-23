// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

// Package roombrief writes the short brief on a deal's Deal Room tab: what the
// room's buyers did and what is waiting on the seller.
//
// Tables owned: none. The brief is written on every request and stored
// nowhere, because a room moves with each buyer's click and a stored brief
// would be the one thing on the tab that is out of date.
//
// PER VIEWER. The input is read through the room's own gated reads, as the
// requesting caller, so the brief can only describe what that caller could
// open on the tab beside it.
//
// DEGRADES RATHER THAN FAILS. With no model lane, a spent budget, or a reply
// the grounding filter refuses, the brief is a deterministic composition of the
// same records, and generated_by names which writer the reader is holding.
//
// EVERY SENTENCE IS CITED OR DROPPED, under the one grounding rule in
// compose/claims: a sentence citing a participant, document or thread the
// input did not carry is dropped whole.
package roombrief
