"use client";

import { useEffect, useMemo, useState } from "react";
import {
  createCalendarEvent,
  deleteCalendarEvent,
  listCalendarEvents,
  updateCalendarEvent,
  type CalendarEvent,
  type CalendarEventInput
} from "@/lib/api";
import { formatDateTime, normalizeErrorMessage, type Flash } from "@/components/file-manager/helpers";
import { useAppContext } from "@/components/app-context-provider";

const reminderValues = ["at_time", "10m", "1h", "1d", "3d"] as const;

type ReminderValue = (typeof reminderValues)[number];

type Props = {
  runWithRefresh: <T>(operation: () => Promise<T>) => Promise<T>;
  showFlash: (tone: Flash["tone"], message: string) => void;
};

type FormState = {
  title: string;
  description: string;
  location: string;
  allDay: boolean;
  startDate: string;
  endDate: string;
  startDatetime: string;
  endDatetime: string;
  reminders: ReminderValue[];
};

const emptyForm: FormState = {
  title: "",
  description: "",
  location: "",
  allDay: false,
  startDate: "",
  endDate: "",
  startDatetime: "",
  endDatetime: "",
  reminders: []
};

function startOfMonth(input: Date): Date {
  return new Date(input.getFullYear(), input.getMonth(), 1);
}

function startOfCalendarGrid(monthDate: Date): Date {
  const monthStart = startOfMonth(monthDate);
  const weekday = monthStart.getDay();
  const mondayIndex = (weekday + 6) % 7;
  return new Date(monthStart.getFullYear(), monthStart.getMonth(), monthStart.getDate() - mondayIndex);
}

function addDays(input: Date, days: number): Date {
  return new Date(input.getFullYear(), input.getMonth(), input.getDate() + days, input.getHours(), input.getMinutes(), input.getSeconds(), input.getMilliseconds());
}

function formatDateKey(input: Date): string {
  const year = input.getFullYear();
  const month = `${input.getMonth() + 1}`.padStart(2, "0");
  const day = `${input.getDate()}`.padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function toLocalDateInput(input: Date): string {
  return formatDateKey(input);
}

function toLocalDatetimeInput(input: Date): string {
  const year = input.getFullYear();
  const month = `${input.getMonth() + 1}`.padStart(2, "0");
  const day = `${input.getDate()}`.padStart(2, "0");
  const hour = `${input.getHours()}`.padStart(2, "0");
  const minute = `${input.getMinutes()}`.padStart(2, "0");
  return `${year}-${month}-${day}T${hour}:${minute}`;
}

function parseLocalDate(value: string): Date | null {
  const parsed = new Date(`${value}T00:00:00`);
  if (Number.isNaN(parsed.getTime())) {
    return null;
  }
  return parsed;
}

function parseLocalDatetime(value: string): Date | null {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return null;
  }
  return parsed;
}

function eventLooksAllDay(start: Date, end: Date): boolean {
  const startsAtMidnight = start.getHours() === 0 && start.getMinutes() === 0;
  const endsAtMidnight = end.getHours() === 0 && end.getMinutes() === 0;
  return startsAtMidnight && endsAtMidnight && end.getTime() > start.getTime();
}

function defaultFormForDate(day: Date): FormState {
  const start = new Date(day.getFullYear(), day.getMonth(), day.getDate(), 9, 0, 0, 0);
  const end = addDays(start, 0);
  end.setHours(10, 0, 0, 0);

  return {
    ...emptyForm,
    startDate: toLocalDateInput(day),
    endDate: toLocalDateInput(day),
    startDatetime: toLocalDatetimeInput(start),
    endDatetime: toLocalDatetimeInput(end)
  };
}

function formFromEvent(event: CalendarEvent): FormState {
  const start = new Date(event.start_datetime);
  const end = new Date(event.end_datetime);
  const allDay = eventLooksAllDay(start, end);

  return {
    title: event.title,
    description: event.description ?? "",
    location: event.location ?? "",
    allDay,
    startDate: toLocalDateInput(start),
    endDate: toLocalDateInput(allDay ? addDays(end, -1) : end),
    startDatetime: toLocalDatetimeInput(start),
    endDatetime: toLocalDatetimeInput(end),
    reminders: event.reminders.filter((value): value is ReminderValue => reminderValues.includes(value as ReminderValue))
  };
}

function validateForm(form: FormState, t: (key: string, fallback?: string, params?: Record<string, string | number>) => string): string | null {
  if (form.title.trim() === "") {
    return t("calendar.validation.titleRequired", "Title is required.");
  }

  if (form.allDay) {
    if (!form.startDate || !form.endDate) {
      return t("calendar.validation.allDayDateRequired", "Select date(s) for all-day event.");
    }
    const start = parseLocalDate(form.startDate);
    const end = parseLocalDate(form.endDate);
    if (!start || !end) {
      return t("calendar.validation.validDate", "Select a valid date.");
    }
    if (end.getTime() < start.getTime()) {
      return t("calendar.validation.endBeforeStartDate", "End date cannot be before start date.");
    }
    return null;
  }

  const start = parseLocalDatetime(form.startDatetime);
  const end = parseLocalDatetime(form.endDatetime);
  if (!start || !end) {
    return t("calendar.validation.validDateTime", "Start/end date-time is invalid.");
  }
  if (end.getTime() <= start.getTime()) {
    return t("calendar.validation.endAfterStart", "End time must be after start time.");
  }
  return null;
}

function toEventInput(form: FormState): CalendarEventInput {
  if (form.allDay) {
    const start = parseLocalDate(form.startDate) ?? new Date();
    const endDate = parseLocalDate(form.endDate) ?? start;
    const endExclusive = addDays(endDate, 1);
    return {
      title: form.title.trim(),
      description: form.description.trim(),
      location: form.location.trim() || null,
      start_datetime: start.toISOString(),
      end_datetime: endExclusive.toISOString(),
      reminders: form.reminders
    };
  }

  const start = parseLocalDatetime(form.startDatetime) ?? new Date();
  const end = parseLocalDatetime(form.endDatetime) ?? addDays(start, 1);
  return {
    title: form.title.trim(),
    description: form.description.trim(),
    location: form.location.trim() || null,
    start_datetime: start.toISOString(),
    end_datetime: end.toISOString(),
    reminders: form.reminders
  };
}

function monthTitle(input: Date, locale: string): string {
  return input.toLocaleDateString(locale, {
    month: "long",
    year: "numeric"
  });
}

export function CalendarPanel({ runWithRefresh, showFlash }: Props) {
  const { locale, t } = useAppContext();
  const [currentMonth, setCurrentMonth] = useState(() => startOfMonth(new Date()));
  const [events, setEvents] = useState<CalendarEvent[]>([]);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [modalOpen, setModalOpen] = useState(false);
  const [editingEventID, setEditingEventID] = useState<string | null>(null);
  const [selectedDate, setSelectedDate] = useState<Date | null>(null);
  const [form, setForm] = useState<FormState>(emptyForm);

  const reminderOptions = useMemo(
    () =>
      [
        { value: "at_time" as const, label: t("calendar.reminder.atTime", "At event time") },
        { value: "10m" as const, label: t("calendar.reminder.10m", "10 minutes before") },
        { value: "1h" as const, label: t("calendar.reminder.1h", "1 hour before") },
        { value: "1d" as const, label: t("calendar.reminder.1d", "1 day before") },
        { value: "3d" as const, label: t("calendar.reminder.3d", "3 days before") }
      ],
    [t]
  );

  const weekdayLabels = useMemo(
    () => [
      t("calendar.weekday.mon", "Mon"),
      t("calendar.weekday.tue", "Tue"),
      t("calendar.weekday.wed", "Wed"),
      t("calendar.weekday.thu", "Thu"),
      t("calendar.weekday.fri", "Fri"),
      t("calendar.weekday.sat", "Sat"),
      t("calendar.weekday.sun", "Sun")
    ],
    [t]
  );

  const gridStart = useMemo(() => startOfCalendarGrid(currentMonth), [currentMonth]);
  const gridDays = useMemo(() => Array.from({ length: 42 }, (_, index) => addDays(gridStart, index)), [gridStart]);
  const rangeStart = useMemo(() => new Date(gridDays[0].getFullYear(), gridDays[0].getMonth(), gridDays[0].getDate(), 0, 0, 0, 0), [gridDays]);
  const rangeEnd = useMemo(() => {
    const last = gridDays[gridDays.length - 1];
    return new Date(last.getFullYear(), last.getMonth(), last.getDate(), 23, 59, 59, 999);
  }, [gridDays]);

  const eventsByDay = useMemo(() => {
    const map = new Map<string, CalendarEvent[]>();
    for (const event of events) {
      const start = new Date(event.start_datetime);
      const key = formatDateKey(start);
      const previous = map.get(key) ?? [];
      previous.push(event);
      map.set(key, previous);
    }
    for (const list of map.values()) {
      list.sort((a, b) => new Date(a.start_datetime).getTime() - new Date(b.start_datetime).getTime());
    }
    return map;
  }, [events]);

  const upcomingEvents = useMemo(() => {
    const now = Date.now();
    return [...events]
      .filter((item) => new Date(item.end_datetime).getTime() >= now)
      .sort((a, b) => new Date(a.start_datetime).getTime() - new Date(b.start_datetime).getTime())
      .slice(0, 8);
  }, [events]);

  const loadEvents = async (monthToLoad = currentMonth) => {
    setLoading(true);
    setError(null);
    try {
      const monthGridStart = startOfCalendarGrid(monthToLoad);
      const monthGridEnd = addDays(monthGridStart, 41);
      const from = new Date(monthGridStart.getFullYear(), monthGridStart.getMonth(), monthGridStart.getDate(), 0, 0, 0, 0);
      const to = new Date(monthGridEnd.getFullYear(), monthGridEnd.getMonth(), monthGridEnd.getDate(), 23, 59, 59, 999);
      const items = await runWithRefresh(() => listCalendarEvents({ from: from.toISOString(), to: to.toISOString() }));
      setEvents(items);
    } catch (err) {
      setError(normalizeErrorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadEvents(currentMonth);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentMonth]);

  const openCreateModal = (day: Date) => {
    setSelectedDate(day);
    setEditingEventID(null);
    setForm(defaultFormForDate(day));
    setModalOpen(true);
  };

  const openEditModal = (event: CalendarEvent) => {
    setSelectedDate(new Date(event.start_datetime));
    setEditingEventID(event.id);
    setForm(formFromEvent(event));
    setModalOpen(true);
  };

  const closeModal = () => {
    if (busy) {
      return;
    }
    setModalOpen(false);
    setEditingEventID(null);
    setSelectedDate(null);
    setForm(emptyForm);
  };

  const saveEvent = async () => {
    const validationError = validateForm(form, t);
    if (validationError) {
      showFlash("error", validationError);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const payload = toEventInput(form);
      if (editingEventID) {
        await runWithRefresh(() => updateCalendarEvent(editingEventID, payload));
        showFlash("success", t("calendar.updated", "Event updated."));
      } else {
        await runWithRefresh(() => createCalendarEvent(payload));
        showFlash("success", t("calendar.created", "Event created."));
      }
      closeModal();
      await loadEvents();
    } catch (err) {
      setError(normalizeErrorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  const removeEditingEvent = async () => {
    if (!editingEventID) {
      return;
    }
    const approved = window.confirm(t("calendar.deleteConfirm", "Delete this event?"));
    if (!approved) {
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await runWithRefresh(() => deleteCalendarEvent(editingEventID));
      showFlash("success", t("calendar.deleted", "Event deleted."));
      closeModal();
      await loadEvents();
    } catch (err) {
      setError(normalizeErrorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_320px]">
      <div className="surface-card rounded-2xl p-4 shadow-sm" data-testid="calendar-month-grid">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
          <div>
            <h2 className="text-lg font-semibold">{t("calendar.title", "Calendar")}</h2>
            <p className="text-xs uppercase tracking-wide text-[var(--text-muted)]">{monthTitle(currentMonth, locale)}</p>
          </div>
          <div className="flex items-center gap-2">
            <button type="button" onClick={() => setCurrentMonth((prev) => startOfMonth(new Date(prev.getFullYear(), prev.getMonth() - 1, 1)))} className="focus-ring rounded-lg border border-[var(--line)] px-3 py-2 text-sm hover:bg-[var(--bg-soft)]">
              <i className="fa-solid fa-chevron-left" />
            </button>
            <button type="button" onClick={() => setCurrentMonth(startOfMonth(new Date()))} className="focus-ring rounded-lg border border-[var(--line)] px-3 py-2 text-sm hover:bg-[var(--bg-soft)]">
              {t("calendar.today", "Today")}
            </button>
            <button type="button" onClick={() => setCurrentMonth((prev) => startOfMonth(new Date(prev.getFullYear(), prev.getMonth() + 1, 1)))} className="focus-ring rounded-lg border border-[var(--line)] px-3 py-2 text-sm hover:bg-[var(--bg-soft)]">
              <i className="fa-solid fa-chevron-right" />
            </button>
          </div>
        </div>

        {error ? <p className="mb-3 rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-500">{error}</p> : null}
        {loading ? <p className="mb-3 text-sm text-[var(--text-muted)]">{t("calendar.loading", "Loading calendar...")}</p> : null}

        <div className="grid grid-cols-7 gap-2 text-center text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">
          {weekdayLabels.map((label) => (
            <div key={label} className="rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] py-2">
              {label}
            </div>
          ))}
        </div>

        <div className="mt-2 grid grid-cols-7 gap-2">
          {gridDays.map((day) => {
            const key = formatDateKey(day);
            const isCurrentMonth = day.getMonth() === currentMonth.getMonth();
            const isToday = key === formatDateKey(new Date());
            const dayEvents = eventsByDay.get(key) ?? [];

            return (
              <button
                key={key}
                type="button"
                data-testid="calendar-day-cell"
                onClick={() => openCreateModal(day)}
                className={`focus-ring min-h-[120px] rounded-xl border p-2 text-left align-top transition ${
                  isCurrentMonth
                    ? "border-[var(--line)] bg-[var(--bg-card)] hover:border-[var(--brand)]/35 hover:shadow-[0_0_0_1px_color-mix(in_srgb,var(--brand)_22%,transparent)]"
                    : "border-[var(--line)] bg-[var(--bg-soft)]/70 text-[var(--text-muted)]"
                }`}
              >
                <div className="mb-1 flex items-center justify-between">
                  <span className={`inline-flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold ${isToday ? "bg-[var(--brand)] text-white" : ""}`}>
                    {day.getDate()}
                  </span>
                  <span className="text-[10px] text-[var(--text-muted)]">{dayEvents.length > 0 ? `${dayEvents.length}` : ""}</span>
                </div>
                <div className="space-y-1">
                  {dayEvents.slice(0, 3).map((event) => (
                    <div
                      key={event.id}
                      className="truncate rounded-md border border-[var(--brand)]/30 bg-[var(--brand-soft)] px-1.5 py-1 text-[11px] font-medium text-[var(--brand)]"
                      onClick={(clickEvent) => {
                        clickEvent.stopPropagation();
                        openEditModal(event);
                      }}
                    >
                      {event.title}
                    </div>
                  ))}
                  {dayEvents.length > 3 ? <p className="text-[11px] text-[var(--text-muted)]">{t("calendar.moreCount", "+{count} more", { count: dayEvents.length - 3 })}</p> : null}
                </div>
              </button>
            );
          })}
        </div>

        <p className="mt-3 text-xs text-[var(--text-muted)]">
          {t("calendar.range", "Range")}: {formatDateTime(rangeStart.toISOString())} - {formatDateTime(rangeEnd.toISOString())}
        </p>
      </div>

      <aside className="space-y-4">
        <div className="surface-card rounded-2xl p-4 shadow-sm">
          <h3 className="text-sm font-semibold uppercase tracking-wide text-[var(--text-muted)]">{t("calendar.upcoming", "Upcoming Events")}</h3>
          {upcomingEvents.length === 0 ? (
            <p className="mt-3 text-sm text-[var(--text-muted)]">{t("calendar.upcomingEmpty", "No upcoming events.")}</p>
          ) : (
            <div className="mt-3 space-y-2">
              {upcomingEvents.map((event) => (
                <button
                  key={event.id}
                  type="button"
                  onClick={() => openEditModal(event)}
                  className="focus-ring block w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] p-2 text-left"
                >
                  <p className="text-sm font-semibold text-[var(--text-main)]">{event.title}</p>
                  <p className="text-xs text-[var(--text-muted)]">{formatDateTime(event.start_datetime)}</p>
                  {event.location ? <p className="text-xs text-[var(--text-muted)]">{event.location}</p> : null}
                </button>
              ))}
            </div>
          )}
        </div>
      </aside>

      {modalOpen ? (
        <div className="modal-overlay fixed inset-0 z-50 grid place-items-center p-4" data-testid="calendar-event-modal">
          <div className="surface-card w-full max-w-xl rounded-2xl p-5">
            <div className="mb-4 flex items-center justify-between gap-2">
              <div>
                <h3 className="text-lg font-semibold">{editingEventID ? t("calendar.editEvent", "Edit Event") : t("calendar.newEvent", "New Event")}</h3>
                {selectedDate ? <p className="text-xs text-[var(--text-muted)]">{selectedDate.toLocaleDateString(locale)}</p> : null}
              </div>
              <button type="button" onClick={closeModal} className="focus-ring rounded-lg px-2 py-1 hover:bg-[var(--bg-soft)]">
                <i className="fa-solid fa-xmark" />
              </button>
            </div>

            <div className="grid gap-3 md:grid-cols-2">
              <input value={form.title} onChange={(event) => setForm((prev) => ({ ...prev, title: event.target.value }))} placeholder={t("calendar.field.title", "Title")} data-testid="calendar-input-title" className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none md:col-span-2" />
              <input value={form.location} onChange={(event) => setForm((prev) => ({ ...prev, location: event.target.value }))} placeholder={t("calendar.field.locationOptional", "Location (optional)")} data-testid="calendar-input-location" className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none md:col-span-2" />
              <label className="inline-flex items-center gap-2 text-sm md:col-span-2">
                <input type="checkbox" checked={form.allDay} onChange={(event) => setForm((prev) => ({ ...prev, allDay: event.target.checked }))} data-testid="calendar-input-all-day" />
                {t("calendar.field.allDay", "All day")}
              </label>

              {form.allDay ? (
                <>
                  <input type="date" value={form.startDate} onChange={(event) => setForm((prev) => ({ ...prev, startDate: event.target.value }))} data-testid="calendar-input-start-date" className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none" />
                  <input type="date" value={form.endDate} onChange={(event) => setForm((prev) => ({ ...prev, endDate: event.target.value }))} data-testid="calendar-input-end-date" className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none" />
                </>
              ) : (
                <>
                  <input type="datetime-local" value={form.startDatetime} onChange={(event) => setForm((prev) => ({ ...prev, startDatetime: event.target.value }))} data-testid="calendar-input-start" className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none" />
                  <input type="datetime-local" value={form.endDatetime} onChange={(event) => setForm((prev) => ({ ...prev, endDatetime: event.target.value }))} data-testid="calendar-input-end" className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none" />
                </>
              )}
            </div>

            <textarea value={form.description} onChange={(event) => setForm((prev) => ({ ...prev, description: event.target.value }))} placeholder={t("calendar.field.description", "Description")} data-testid="calendar-input-description" className="focus-ring mt-3 h-24 w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none" />

            <div className="mt-3">
              <p className="mb-2 text-xs uppercase tracking-wide text-[var(--text-muted)]">{t("calendar.reminders", "Reminders")}</p>
              <div className="grid gap-2 sm:grid-cols-2">
                {reminderOptions.map((option) => (
                  <label key={option.value} className="inline-flex items-center gap-2 text-sm">
                    <input
                      type="checkbox"
                      checked={form.reminders.includes(option.value)}
                      onChange={(event) =>
                        setForm((prev) => ({
                          ...prev,
                          reminders: event.target.checked
                            ? [...prev.reminders, option.value]
                            : prev.reminders.filter((item) => item !== option.value)
                        }))
                      }
                      data-testid={`calendar-reminder-${option.value}`}
                    />
                    {option.label}
                  </label>
                ))}
              </div>
            </div>

            <div className="mt-4 flex flex-wrap items-center justify-between gap-2">
              <div>
                {editingEventID ? (
                  <button type="button" onClick={() => void removeEditingEvent()} disabled={busy} data-testid="calendar-delete-button" className="focus-ring rounded-lg border border-red-500/40 px-3 py-2 text-sm text-red-500 hover:bg-red-500/10 disabled:opacity-60">
                    {t("common.delete", "Delete")}
                  </button>
                ) : null}
              </div>
              <div className="flex items-center gap-2">
                <button type="button" onClick={closeModal} className="focus-ring rounded-lg border border-[var(--line)] px-3 py-2 text-sm hover:bg-[var(--bg-soft)]">
                  {t("common.cancel", "Cancel")}
                </button>
                <button type="button" onClick={() => void saveEvent()} disabled={busy} data-testid="calendar-save-button" className="focus-ring rounded-lg bg-[var(--brand)] px-4 py-2 text-sm font-semibold text-white hover:brightness-110 disabled:opacity-60">
                  {editingEventID ? t("common.update", "Update") : t("common.create", "Create")}
                </button>
              </div>
            </div>
          </div>
        </div>
      ) : null}
    </section>
  );
}
