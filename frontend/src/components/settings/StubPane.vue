<script setup lang="ts">
import { computed } from 'vue'
import SettingsRow from './SettingsRow.vue'
import SettingsSection from './SettingsSection.vue'
import Segment from '../ui/Segment.vue'
import ValueChip from '../ui/ValueChip.vue'
import { Button } from '../ui/button'
import { Switch } from '../ui/switch'
import { useMessages } from '../../i18n'
import type { CategoryId } from './categories'

// A category the design draws and the app cannot fill: every row is the drawing's, in its words and
// its controls, and every control is switched off. The note under the first card says why there is
// nothing behind it — a screen that looked operable and did nothing would be worse than no screen.
//
// The rows are the drawing's own numbers (a proxy mode of "System", two client certificates, a plan
// of four seats) because they are what the design says the screen will hold; none of them is read
// from anywhere, and the whole pane is inert.
const props = defineProps<{ category: CategoryId }>()

const { t } = useMessages()

type Control =
  | { kind: 'toggle'; on: boolean }
  | { kind: 'segment'; options: { value: string; label: string }[]; value: string }
  | { kind: 'value'; value: string }
  | { kind: 'button'; label: string; primary?: boolean }

type Row = { title: string; note: string; control: Control }
type Section = { label: string; note: string; rows: Row[] }

const sections = computed<Section[]>(() => {
  switch (props.category) {
    case 'account':
      return [
        {
          label: t('settings.sec.account'),
          note: t('settings.stub.account.note'),
          rows: [
            {
              title: t('settings.stub.account.out'),
              note: t('settings.stub.account.outNote'),
              control: { kind: 'button', label: t('settings.stub.account.outAction'), primary: true },
            },
            {
              title: t('settings.stub.account.server'),
              note: t('settings.stub.account.serverNote'),
              control: { kind: 'value', value: 'app.jsoninspector.dev' },
            },
          ],
        },
      ]
    case 'proxy':
      return [
        {
          label: t('settings.sec.proxy'),
          note: t('settings.stub.proxy.note'),
          rows: [
            {
              title: t('settings.stub.proxy.mode'),
              note: '',
              control: {
                kind: 'segment',
                value: 'system',
                options: [
                  { value: 'off', label: t('settings.stub.proxy.modeOff') },
                  { value: 'system', label: t('settings.stub.proxy.modeSystem') },
                  { value: 'manual', label: t('settings.stub.proxy.modeManual') },
                ],
              },
            },
            {
              title: t('settings.stub.proxy.address'),
              note: t('settings.stub.proxy.addressNote'),
              control: { kind: 'value', value: '127.0.0.1:8080' },
            },
            {
              title: t('settings.stub.proxy.bypass'),
              note: t('settings.stub.proxy.bypassNote'),
              control: { kind: 'button', label: t('settings.stub.proxy.bypassAction') },
            },
          ],
        },
        {
          label: t('settings.sec.certificates'),
          note: t('settings.stub.proxy.certsNote'),
          rows: [
            { title: t('settings.stub.proxy.ssl'), note: '', control: { kind: 'toggle', on: true } },
            {
              title: t('settings.stub.proxy.clients'),
              note: t('settings.stub.proxy.clientsNote'),
              control: { kind: 'button', label: t('settings.stub.proxy.clientsAction') },
            },
            {
              title: t('settings.stub.proxy.selfSigned'),
              note: '',
              control: { kind: 'toggle', on: true },
            },
          ],
        },
      ]
    case 'security':
      return [
        {
          label: t('settings.sec.secrets'),
          note: t('settings.stub.security.secretsNote'),
          rows: [
            {
              title: t('settings.stub.security.keychain'),
              note: t('settings.stub.security.keychainNote'),
              control: { kind: 'toggle', on: true },
            },
            {
              title: t('settings.stub.security.mask'),
              note: '',
              control: { kind: 'toggle', on: true },
            },
            {
              title: t('settings.stub.security.reveal'),
              note: '',
              control: {
                kind: 'segment',
                value: 'once',
                options: [
                  { value: 'once', label: t('settings.stub.security.revealOnce') },
                  { value: 'always', label: t('settings.stub.security.revealAlways') },
                ],
              },
            },
          ],
        },
        {
          label: t('settings.sec.lock'),
          note: t('settings.stub.security.lockNote'),
          rows: [
            {
              title: t('settings.stub.security.password'),
              note: t('settings.stub.security.passwordNote'),
              control: {
                kind: 'button',
                label: t('settings.stub.security.passwordAction'),
                primary: true,
              },
            },
            {
              title: t('settings.stub.security.lockAfter'),
              note: '',
              control: {
                kind: 'segment',
                value: '15',
                options: [
                  { value: '5', label: t('settings.stub.security.lockAfter5') },
                  { value: '15', label: t('settings.stub.security.lockAfter15') },
                  { value: 'never', label: t('settings.stub.security.lockAfterNever') },
                ],
              },
            },
            {
              title: t('settings.stub.security.touchId'),
              note: t('settings.stub.security.touchIdNote'),
              control: { kind: 'toggle', on: false },
            },
          ],
        },
        {
          label: t('settings.sec.traffic'),
          note: t('settings.stub.security.trafficNote'),
          rows: [
            {
              title: t('settings.stub.security.clearOnClose'),
              note: '',
              control: { kind: 'toggle', on: true },
            },
            {
              title: t('settings.stub.security.stripAuth'),
              note: '',
              control: { kind: 'toggle', on: true },
            },
          ],
        },
      ]
    default:
      return [
        {
          label: t('settings.sec.files'),
          note: t('settings.stub.export.note'),
          rows: [
            {
              title: t('settings.stub.export.format'),
              note: '',
              control: {
                kind: 'segment',
                value: 'json',
                // Format names are the formats' own: they are spelled the same in every language.
                options: [
                  { value: 'json', label: 'JSON' },
                  { value: 'openapi', label: 'OpenAPI' },
                  { value: 'har', label: 'HAR' },
                ],
              },
            },
            {
              title: t('settings.stub.export.pretty'),
              note: '',
              control: { kind: 'toggle', on: true },
            },
            {
              title: t('settings.stub.export.location'),
              note: t('settings.stub.export.locationNote'),
              control: { kind: 'value', value: '~/Documents/JSON Inspector' },
            },
          ],
        },
        {
          label: t('settings.sec.contents'),
          note: t('settings.stub.export.contentsNote'),
          rows: [
            {
              title: t('settings.stub.export.secrets'),
              note: t('settings.stub.export.secretsNote'),
              control: { kind: 'toggle', on: false },
            },
            {
              title: t('settings.stub.export.history'),
              note: '',
              control: { kind: 'toggle', on: false },
            },
            {
              title: t('settings.stub.export.scripts'),
              note: '',
              control: { kind: 'toggle', on: true },
            },
          ],
        },
      ]
  }
})
</script>

<template>
  <div class="pane">
    <SettingsSection
      v-for="section in sections"
      :key="section.label"
      :label="section.label"
      :note="section.note"
    >
      <SettingsRow
        v-for="row in section.rows"
        :key="row.title"
        :title="row.title"
        :note="row.note"
      >
        <Switch v-if="row.control.kind === 'toggle'" :model-value="row.control.on" disabled />
        <Segment
          v-else-if="row.control.kind === 'segment'"
          size="sm"
          :grow="false"
          :options="row.control.options"
          :value="row.control.value"
          disabled
        />
        <ValueChip
          v-else-if="row.control.kind === 'value'"
          class="dim"
          :value="row.control.value"
          mono
        />
        <Button
          v-else
          :variant="row.control.primary ? 'primary' : 'outline'"
          size="field"
          disabled
        >
          {{ row.control.label }}
        </Button>
      </SettingsRow>
    </SettingsSection>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.pane {
  @apply flex flex-col;
  gap: 18px;
}

/* A value chip has nothing to switch off — it takes no clicks — so the dimming the other controls do
   with `disabled` is written here. */
.dim {
  @apply opacity-50;
}
</style>
