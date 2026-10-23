// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

// A month of days: `DateInput`'s popup, and shown open where the month must stay
// in view while the rest of a choice is read. Month and chosen day are the caller's.

import { ChevronLeft, ChevronRight } from "lucide-react";
import { type KeyboardEvent, useEffect, useRef, useState } from "react";
import type { ISODate } from "../format/datepattern";
import {
  formatNumber,
  fullDayName,
  monthAndYear,
  weekdayInitial,
} from "../format/format";
import { type Weekday, weekStart } from "../format/weekstart";
import type { Locale } from "../i18n";
import { useT } from "../i18n";
import { Button } from "./atoms";
import "./calendar.css";

export type ISODay = ISODate;

/** isoDay is a `Date` as the day it falls on in the reader's own zone. */
export function isoDay(at: Date): ISODay {
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${at.getFullYear()}-${pad(at.getMonth() + 1)}-${pad(at.getDate())}` as ISODay;
}

/** The local midnight a calendar day starts at. */
export function dateOf(day: ISODay): Date {
  const [year, month, date] = day.split("-").map(Number);
  return new Date(year ?? 0, (month ?? 1) - 1, date ?? 1);
}

// Always six weeks, so the controls under the grid stay put while the reader pages.
const WEEKS = 6;

/** The days the grid draws, from the week start before the 1st. */
export function monthGrid(month: Date, start: Weekday): Date[] {
  const first = new Date(month.getFullYear(), month.getMonth(), 1);
  const lead = (first.getDay() - start + 7) % 7;
  return Array.from(
    { length: WEEKS * 7 },
    (_, index) =>
      new Date(first.getFullYear(), first.getMonth(), 1 - lead + index),
  );
}

/** One month with one day marked; `today` is the caller's, so a test can say what it draws. */
export function Calendar({
  month,
  onMonthChange,
  selected,
  onSelect,
  today,
  locale,
  refusal,
}: Readonly<{
  month: Date;
  onMonthChange: (next: Date) => void;
  selected: ISODay | "";
  onSelect: (next: ISODay) => void;
  today: Date;
  locale: Locale;
  /** Why a day cannot be chosen, translated; it joins the day's name. */
  refusal?: (day: ISODay) => string | undefined;
}>) {
  const t = useT();
  const start = weekStart(locale);
  const days = monthGrid(month, start);
  const [cursor, setCursor] = useState<ISODay | null>(null);
  const grid = useRef<HTMLDivElement | null>(null);
  const wantsFocus = useRef(false);
  const tabStop = tabStopOf(month, [cursor, selected, isoDay(today)]);

  useEffect(() => {
    if (!wantsFocus.current) {
      return;
    }
    wantsFocus.current = false;
    grid.current
      ?.querySelector<HTMLElement>(`[data-day="${tabStop}"]`)
      ?.focus();
  }, [tabStop]);

  const move = (event: KeyboardEvent, from: Date) => {
    const next = stepFrom(from, event.key, event.shiftKey, start);
    if (!next) {
      return;
    }
    event.preventDefault();
    if (isoDay(next) === isoDay(from)) {
      return;
    }
    wantsFocus.current = true;
    setCursor(isoDay(next));
    if (!sameMonth(next, month)) {
      onMonthChange(new Date(next.getFullYear(), next.getMonth(), 1));
    }
  };

  return (
    <div className="calendar">
      <div className="calendar-head">
        <span className="calendar-month t-body" aria-live="polite">
          {monthAndYear(month, locale)}
        </span>
        <Button
          iconOnly
          aria-label={t("calendar.previousMonth")}
          onClick={() => onMonthChange(shiftMonth(month, -1))}
        >
          <ChevronLeft aria-hidden="true" />
        </Button>
        <Button
          iconOnly
          aria-label={t("calendar.nextMonth")}
          onClick={() => onMonthChange(shiftMonth(month, 1))}
        >
          <ChevronRight aria-hidden="true" />
        </Button>
      </div>
      <div className="calendar-grid" ref={grid}>
        {days.slice(0, 7).map((day) => (
          <span key={`weekday-${day.getDay()}`} aria-hidden="true">
            {weekdayInitial(day, locale)}
          </span>
        ))}
        {days.map((day) => (
          <CalendarDay
            key={isoDay(day)}
            day={day}
            inMonth={sameMonth(day, month)}
            isToday={isoDay(day) === isoDay(today)}
            isSelected={isoDay(day) === selected}
            isTabStop={isoDay(day) === tabStop}
            refusal={refusal?.(isoDay(day))}
            onSelect={onSelect}
            onMove={move}
            locale={locale}
          />
        ))}
      </div>
    </div>
  );
}

function CalendarDay({
  day,
  inMonth,
  isToday,
  isSelected,
  isTabStop,
  refusal,
  onSelect,
  onMove,
  locale,
}: Readonly<{
  day: Date;
  inMonth: boolean;
  isToday: boolean;
  isSelected: boolean;
  isTabStop: boolean;
  refusal: string | undefined;
  onSelect: (next: ISODay) => void;
  onMove: (event: KeyboardEvent, from: Date) => void;
  locale: Locale;
}>) {
  const classes = [
    "calendar-day",
    inMonth ? "" : "calendar-day-outside",
    isToday ? "calendar-day-today" : "",
  ]
    .filter(Boolean)
    .join(" ");
  return (
    <button
      type="button"
      className={classes}
      data-day={isoDay(day)}
      tabIndex={isTabStop ? 0 : -1}
      aria-pressed={isSelected}
      aria-label={
        refusal
          ? `${fullDayName(day, locale)}, ${refusal}`
          : fullDayName(day, locale)
      }
      // Not `disabled`: a disabled day takes no focus, so the arrows would skip it.
      aria-disabled={refusal === undefined ? undefined : true}
      onKeyDown={(event) => onMove(event, day)}
      onClick={() => {
        if (refusal === undefined) {
          onSelect(isoDay(day));
        }
      }}
    >
      {formatNumber(day.getDate(), locale)}
    </button>
  );
}

function tabStopOf(
  month: Date,
  candidates: ReadonlyArray<ISODay | "" | null>,
): ISODay {
  const inMonth = candidates.find(
    (day): day is ISODay => !!day && sameMonth(dateOf(day), month),
  );
  return inMonth ?? isoDay(new Date(month.getFullYear(), month.getMonth(), 1));
}

function stepFrom(
  from: Date,
  key: string,
  shift: boolean,
  start: Weekday,
): Date | null {
  const by = (days: number) =>
    new Date(from.getFullYear(), from.getMonth(), from.getDate() + days);
  const intoWeek = (from.getDay() - start + 7) % 7;
  switch (key) {
    case "ArrowLeft":
      return by(-1);
    case "ArrowRight":
      return by(1);
    case "ArrowUp":
      return by(-7);
    case "ArrowDown":
      return by(7);
    case "Home":
      return by(-intoWeek);
    case "End":
      return by(6 - intoWeek);
    case "PageUp":
      return sameDayMonthsAway(from, shift ? -12 : -1);
    case "PageDown":
      return sameDayMonthsAway(from, shift ? 12 : 1);
    default:
      return null;
  }
}

// The 31st a month away lands on that month's last day, not in the month after.
function sameDayMonthsAway(from: Date, by: number): Date {
  const target = new Date(from.getFullYear(), from.getMonth() + by, 1);
  const last = new Date(
    target.getFullYear(),
    target.getMonth() + 1,
    0,
  ).getDate();
  return new Date(
    target.getFullYear(),
    target.getMonth(),
    Math.min(from.getDate(), last),
  );
}

function sameMonth(a: Date, b: Date): boolean {
  return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth();
}

function shiftMonth(month: Date, by: number): Date {
  return new Date(month.getFullYear(), month.getMonth() + by, 1);
}
