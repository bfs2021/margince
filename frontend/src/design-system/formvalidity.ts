// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

// DateInput reports text it cannot read as "", so a save past it erases the day.
// Not checkValidity(): native step and required rules belong to the caller and server.
export function refuseUnreadableField(from: Element): boolean {
  const unread = Array.from(scopeOf(from).querySelectorAll("input")).find(
    (field) => field.validity.customError,
  );
  unread?.reportValidity();
  return unread !== undefined;
}

// Outside a form or dialog, the nearest box around `from` that holds a date field.
function scopeOf(from: Element): Element {
  const bounded = from.closest('form, [role="dialog"]');
  if (bounded) {
    return bounded;
  }
  let scope = from;
  while (!scope.querySelector(".date-input") && scope.parentElement) {
    scope = scope.parentElement;
  }
  return scope;
}
