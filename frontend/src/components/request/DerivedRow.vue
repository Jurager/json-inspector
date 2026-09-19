<script setup lang="ts">
import Icon from '../ui/Icon.vue'
import TokenField from '../ui/TokenField.vue'
import { IconButton } from '../ui/button'
import { useAuthSchemes } from '../../composables/useAuthSchemes'
import { useMessages } from '../../i18n'
import { RowKind, type ProjectedRow } from '../../../bindings/json-inspector/internal/domain'

// A row the authorization put in the list rather than a person. It sits in the same grid as the rows
// around it so the columns line up, and it has no checkbox: there is nothing here to switch off — a
// credential that does not go out is an authorization that is not set.
//
// Its name and value are the scheme's fields seen from this side. Editing one is editing the field
// behind it, which is why the row carries no id and is addressed by what it is instead.
const props = defineProps<{ row: ProjectedRow }>()

const emit = defineEmits<{
  // The list travels with the edit: a scheme can put the same credential in a header or in the
  // query, and the row is addressed by where it is and what it is called.
  patch: [target: RowKind, name: string, value: string]
  remove: []
}>()

const { t } = useMessages()
const { schemeOf } = useAuthSchemes()

const scheme = () => schemeOf(props.row.from)

// A value that is not edited here still has to be coloured the way every other value is: numbers take
// the number token, everything else the string one.
function valueClass(v: string): string {
  return /^-?\d+(\.\d+)?$/.test(v.trim()) ? 'num' : 'str'
}
</script>

<template>
  <div class="row derived" :title="t('request.auth.derivedFrom', { scheme: scheme() ? t(scheme()!.label) : row.from })">
    <span class="mark"><Icon name="link" :size="13" /></span>

    <input
      v-if="row.editable"
      :value="row.name"
      class="row-input mono"
      :placeholder="t('request.placeholderName')"
      spellcheck="false"
      @input="emit('patch', row.target, ($event.target as HTMLInputElement).value, row.value)"
    />
    <span v-else class="row-text mono">{{ row.name }}</span>

    <TokenField
      v-if="row.editable"
      :value="row.value"
      :placeholder="t('request.placeholderValue')"
      @change="emit('patch', row.target, row.name, $event)"
    />
    <!-- The cell the value column is drawn in, kept for a row nobody can edit: the grid has to see
         the same shape either way, or this row's columns would not line up with the others'. -->
    <div v-else class="row-cell">
      <span class="row-input mono" :class="valueClass(row.value)">{{ row.value }}</span>
    </div>

    <IconButton
      variant="danger"
      size="xl"
      :hint="t('request.auth.derivedRemove')"
      @click.stop="emit('remove')"
    >
      <Icon name="trash" :size="14" :stroke-width="1.8" />
    </IconButton>
  </div>
</template>

<style scoped>
@reference "../../style.css";

/* The same grid the rows around it stand on — the columns have to line up with theirs, which is the
   whole reason this row is drawn at all. */
.row {
  @apply grid grid-cols-[24px_160px_minmax(0,1fr)_28px] gap-2 items-center h-[34px] px-1.5 rounded-lg;
}

/* The mark is the checkbox column of the rows around it. A projected row is not switched on and off,
   so the column holds the one thing it does say: that it is here because something else is. */
.mark {
  @apply flex items-center justify-center text-accent;
}

.row-text {
  @apply min-w-0 text-[13px] truncate;
  color: var(--text-secondary);
}

.row-input {
  @apply min-w-0 bg-transparent border-0 outline-none text-[13px] p-0 rounded-sm;
  font-family: var(--mono);
  color: var(--text);
}

.row-input:focus {
  background: var(--bg-inset);
}

/* The cell the value stands in when nobody can edit it: the grid has to see the same column either
   way, so this keeps the shape TokenField gives the editable one. */
.row-cell {
  @apply relative flex min-w-0;
}

.row-input.str {
  color: var(--tok-str);
}

.row-input.num {
  color: var(--tok-num);
}
</style>
