<script setup lang="ts">
import { computed, onMounted } from 'vue'
import SettingsRow from './SettingsRow.vue'
import SettingsSection from './SettingsSection.vue'
import Segment from '../ui/Segment.vue'
import ValueChip from '../ui/ValueChip.vue'
import { Theme } from '../../../bindings/json-inspector/internal/domain'
import { ListSide } from '../../../bindings/json-inspector/internal/domain'
import { useSettings } from '../../composables/useSettings'
import { useTheme } from '../../composables/useTheme'
import { useMessages } from '../../i18n'

// The palette, and where the window's own panels are: both are choices of the drawing's, and the
// widths are read rather than typed — the panel is dragged, and a number nobody can see being
// measured is what this row is for.
const { t } = useMessages()
const { settings, loadSettings, setLayout } = useSettings()
const { theme, setTheme } = useTheme()

// The handoff's order: light, dark, system. The words are the title bar's own — one control, one set
// of names, whichever window it is drawn in.
const themes = computed(() => [
  { value: Theme.ThemeLight, label: t('theme.light') },
  { value: Theme.ThemeDark, label: t('theme.dark') },
  { value: Theme.ThemeSystem, label: t('theme.system') },
])

const sides = computed(() => [
  { value: ListSide.ListSideLeft, label: t('settings.listSideLeft') },
  { value: ListSide.ListSideRight, label: t('settings.listSideRight') },
  { value: ListSide.ListSideHidden, label: t('settings.listSideHidden') },
])

const listSide = computed(() => settings.value?.listSide ?? ListSide.ListSideLeft)
const sideWidth = computed(() => `${settings.value?.sideWidth ?? 288} px`)
const inspectorWidth = computed(() => `${settings.value?.inspectorWidth ?? 300} px`)

onMounted(() => {
  void loadSettings()
})
</script>

<template>
  <div class="pane">
    <SettingsSection :label="t('settings.sec.theme')">
      <SettingsRow :title="t('settings.theme')">
        <Segment size="sm"
          :grow="false" :options="themes" :value="theme" @pick="setTheme($event as Theme)" />
      </SettingsRow>
    </SettingsSection>

    <SettingsSection :label="t('settings.sec.layout')" :note="t('settings.layoutNote')">
      <SettingsRow :title="t('settings.listSide')">
        <Segment
          size="sm"
          :grow="false"
          :options="sides"
          :value="listSide"
          @pick="setLayout({ listSide: $event as ListSide })"
        />
      </SettingsRow>
      <SettingsRow :title="t('settings.sideWidth')">
        <ValueChip :value="sideWidth" />
      </SettingsRow>
      <SettingsRow :title="t('settings.inspectorWidth')">
        <ValueChip :value="inspectorWidth" />
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
</style>
