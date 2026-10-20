// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import type { TimelineFilters } from "../../design-system/recordtimeline";
import { StoryProviders } from "../../testing/story-utils";
import { useHistoryDials } from "./historydials";
import type { TimelineFilter } from "./recordchronology";

// The dials a History card carries: the cuts in its head, and the search and
// range under it once opened. Live, so the pressed cut is the reader's own.
function Live(
  props: Readonly<{ initial: TimelineFilter; filters?: TimelineFilters }>,
) {
  const [filter, setFilter] = useState<TimelineFilter>(props.initial);
  const [filters, setFilters] = useState<TimelineFilters>(props.filters ?? {});
  const dials = useHistoryDials({
    filter,
    onFilter: setFilter,
    filters,
    onFilters: setFilters,
  });
  return (
    <StoryProviders>
      {dials.cuts}
      {dials.narrowing}
    </StoryProviders>
  );
}

const meta: Meta<typeof Live> = {
  title: "Records/Record 360/History dials",
  component: Live,
};
export default meta;
type Story = StoryObj<typeof Live>;

/** The whole record, nothing narrowed. */
export const All: Story = { render: () => <Live initial="all" /> };

/** One kind pressed: the cut is a server parameter on the activity list. */
export const OneKind: Story = {
  render: () => <Live initial="all" filters={{ kind: "meeting" }} />,
};

/** A search left set keeps its row open, so the list never reads as
 *  narrowed by nothing. */
export const Searching: Story = {
  render: () => <Live initial="all" filters={{ q: "renewal" }} />,
};

/** Changes: the other feed, with no search of its own. */
export const Changes: Story = { render: () => <Live initial="changes" /> };

/** The cuts in dark. */
export const AllDark: Story = { ...All, globals: { theme: "dark" } };
