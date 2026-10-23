// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

import { CalendarDays } from "lucide-react";
import {
  type ComponentPropsWithRef,
  type RefObject,
  useEffect,
  useId,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";
import {
  type DatePattern,
  datePattern,
  formatISODate,
  type ISODate,
  parseTypedDate,
  patternPlaceholder,
} from "../format/datepattern";
import { useDateTimePreferences } from "../format/preferences";
import { type Locale, useLocale, useT } from "../i18n";
import { useAnchoredToTrigger } from "./anchored";
import { useDismissOnOutsidePress } from "./anchoredpopup";
import { Button } from "./button";
import { Calendar, dateOf, isoDay } from "./calendar";
import { useDialogFocus } from "./dialogfocus";
import { IconAction } from "./iconaction";
import "./dateinput.css";

export {
  type ISODate,
  isISODate,
  isoOrBlank,
} from "../format/datepattern";

/** What `onChange` hears: the day as `YYYY-MM-DD`, or `""` once cleared. */
export type DateInputChange = Readonly<{
  target: Readonly<{ value: ISODate | "" }>;
}>;

/**
 * The text box's own props, with the date narrowed to `YYYY-MM-DD`. `today` is the
 * day the calendar marks and Today picks: pass the day a `min`/`max` counts from.
 */
export type DateInputProps = Omit<
  ComponentPropsWithRef<"input">,
  "type" | "value" | "defaultValue" | "onChange" | "min" | "max"
> &
  Readonly<{
    value?: ISODate | "";
    defaultValue?: ISODate | "";
    onChange?: (event: DateInputChange) => void;
    min?: ISODate;
    max?: ISODate;
    today?: ISODate;
  }>;

// `base` is the value before the reader began typing, `reported` what the text
// last told the caller; the text stays while the value is either one.
type Draft = Readonly<{
  text: string;
  base: ISODate | "";
  reported: ISODate | "";
  problem?: string;
}>;

/**
 * The one date field. The caller always holds the box's day, or `""` while the
 * text reads as none; then the box fails `checkValidity()`, and on blur shows `aria-invalid`.
 */
export function DateInput({
  value,
  defaultValue,
  onChange,
  min,
  max,
  today,
  className,
  disabled,
  readOnly,
  name,
  placeholder,
  onBlur,
  onKeyDown,
  ref,
  "aria-invalid": ariaInvalid,
  ...box
}: DateInputProps) {
  const t = useT();
  const { locale } = useLocale();
  const { dateFormat } = useDateTimePreferences();
  const pattern = datePattern(locale, dateFormat);
  const [own, setOwn] = useState<ISODate | "">(defaultValue ?? "");
  const current = value ?? own;
  const [draft, setDraft] = useState<Draft | null>(null);
  const live =
    draft && (current === draft.base || current === draft.reported)
      ? draft
      : null;
  const [openedOn, setOpenedOn] = useState<ISODate | null>(null);
  const shell = useRef<HTMLSpanElement | null>(null);
  const input = useRef<HTMLInputElement | null>(null);
  const popupId = useId();
  const refusal = (day: ISODate) => outsideRange(day, min, max, pattern, t);

  const unread = live
    ? readTyped(live.text, pattern, refusal, t).problem
    : undefined;
  useEffect(() => {
    input.current?.setCustomValidity(unread ?? "");
  }, [unread]);

  const report = (next: ISODate | "") => {
    if (value === undefined) {
      setOwn(next);
    }
    if (next !== current) {
      onChange?.({ target: { value: next } });
    }
  };
  const settle = (next: ISODate | "") => {
    setDraft(null);
    report(next);
  };
  const commit = () => {
    if (!live) {
      return;
    }
    const read = readTyped(live.text, pattern, refusal, t);
    if (read.problem === undefined) {
      settle(read.day);
    } else {
      setDraft({ ...live, reported: "", problem: read.problem });
      report("");
    }
  };
  const type = (text: string) => {
    const read = readTyped(text, pattern, refusal, t);
    const meaning = read.problem === undefined ? read.day : "";
    setDraft({ text, base: live ? live.base : current, reported: meaning });
    report(meaning);
  };

  return (
    <span
      ref={shell}
      className={["field-shell", "date-input", className]
        .filter(Boolean)
        .join(" ")}
    >
      <input
        {...box}
        ref={(node) => {
          input.current = node;
          if (typeof ref === "function") {
            ref(node);
          } else if (ref) {
            ref.current = node;
          }
        }}
        type="text"
        className="input"
        autoComplete="off"
        disabled={disabled}
        readOnly={readOnly}
        placeholder={placeholder ?? placeholderOf(pattern, t)}
        aria-invalid={live?.problem === undefined ? ariaInvalid : true}
        value={live ? live.text : shown(current, pattern)}
        onChange={(event) => type(event.target.value)}
        onBlur={(event) => {
          commit();
          onBlur?.(event);
        }}
        onKeyDown={(event) => {
          if (event.key === "Enter") {
            commit();
          }
          onKeyDown?.(event);
        }}
      />
      <IconAction
        label={t("calendar.chooseDate")}
        icon={<CalendarDays aria-hidden="true" />}
        disabled={disabled || readOnly}
        opensDialog
        disclosure={{ expanded: openedOn !== null, controls: popupId }}
        onClick={() =>
          setOpenedOn((opened) =>
            opened ? null : (today ?? isoDay(new Date())),
          )
        }
      />
      {name !== undefined && (
        <input type="hidden" name={name} value={current} disabled={disabled} />
      )}
      {openedOn && (
        <DatePopup
          id={popupId}
          anchor={shell}
          selected={current}
          today={openedOn}
          locale={locale}
          refusal={refusal}
          onPick={(next) => {
            settle(next);
            setOpenedOn(null);
          }}
          onClose={() => setOpenedOn(null)}
        />
      )}
    </span>
  );
}

type Translate = ReturnType<typeof useT>;

function DatePopup({
  id,
  anchor,
  selected,
  today,
  locale,
  refusal,
  onPick,
  onClose,
}: Readonly<{
  id: string;
  anchor: RefObject<HTMLSpanElement | null>;
  selected: ISODate | "";
  today: ISODate;
  locale: Locale;
  refusal: (day: ISODate) => string | undefined;
  onPick: (next: ISODate | "") => void;
  onClose: () => void;
}>) {
  const t = useT();
  const panel = useRef<HTMLDivElement | null>(null);
  const [month, setMonth] = useState(() => dateOf(selected || today));
  const at = useAnchoredToTrigger(true, anchor, panel, "end");
  useDismissOnOutsidePress(true, onClose, anchor, panel);
  useDialogFocus({
    open: true,
    onClose,
    container: panel,
    initialFocusTo: () =>
      panel.current?.querySelector<HTMLElement>('[data-day][tabindex="0"]') ??
      null,
    returnFocusTo: () =>
      anchor.current?.querySelector<HTMLElement>("button") ?? null,
  });
  const todayRefused = refusal(today) !== undefined;

  return createPortal(
    <div
      ref={panel}
      id={id}
      role="dialog"
      aria-modal="true"
      aria-label={t("calendar.chooseDate")}
      className="date-popup"
      style={{
        top: `${at.top}px`,
        left: `${at.left}px`,
        maxHeight: `${at.maxHeight}px`,
      }}
    >
      <Calendar
        month={month}
        onMonthChange={setMonth}
        selected={selected}
        onSelect={onPick}
        today={dateOf(today)}
        locale={locale}
        refusal={refusal}
      />
      <div className="date-popup-foot">
        <Button disabled={todayRefused} onClick={() => onPick(today)}>
          {t("calendar.today")}
        </Button>
        <Button onClick={() => onPick("")}>{t("calendar.clear")}</Button>
      </div>
    </div>,
    document.body,
  );
}

function shown(day: ISODate | "", pattern: DatePattern): string {
  return day === "" ? "" : formatISODate(day, pattern);
}

function placeholderOf(pattern: DatePattern, t: Translate): string {
  return patternPlaceholder(pattern, {
    day: t("calendar.patternDay"),
    month: t("calendar.patternMonth"),
    year: t("calendar.patternYear"),
  });
}

function outsideRange(
  day: ISODate,
  min: ISODate | undefined,
  max: ISODate | undefined,
  pattern: DatePattern,
  t: Translate,
): string | undefined {
  if (min && day < min) {
    return t("calendar.earliest", { date: formatISODate(min, pattern) });
  }
  if (max && day > max) {
    return t("calendar.latest", { date: formatISODate(max, pattern) });
  }
  return undefined;
}

function readTyped(
  text: string,
  pattern: DatePattern,
  refusal: (day: ISODate) => string | undefined,
  t: Translate,
):
  | Readonly<{ day: ISODate | ""; problem?: undefined }>
  | Readonly<{ problem: string }> {
  if (text.trim() === "") {
    return { day: "" };
  }
  const day = parseTypedDate(text, pattern);
  if (day === null) {
    return {
      problem: t("calendar.invalidDate", {
        pattern: placeholderOf(pattern, t),
      }),
    };
  }
  const refused = refusal(day);
  return refused === undefined ? { day } : { problem: refused };
}
