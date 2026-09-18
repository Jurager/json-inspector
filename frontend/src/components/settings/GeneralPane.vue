<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import SettingsRow from './SettingsRow.vue'
import SettingsSection from './SettingsSection.vue'
import { Button } from '../ui/button'
import { Switch } from '../ui/switch'
import Select from '../ui/Select.vue'
import ValueChip from '../ui/ValueChip.vue'
import { SystemService, WorkspaceService } from '../../../bindings/json-inspector/internal/transport/wails'
import { Language } from '../../../bindings/json-inspector/internal/domain'
import { useSettings } from '../../composables/useSettings'
import { useLocale } from '../../composables/useLocale'
import { useMessages } from '../../i18n'
import { workspaceName } from '../../stores/workspaces'

// What the app does when it starts, what the raw viewer does with a long line, which language the
// window is written in, and where on the disk all of it lives.
const { t } = useMessages()
const { settings, loadSettings, setEditor, setReopenWorkspace } = useSettings()
const { language, setLanguage } = useLocale()

const LANGUAGES = [
  { value: Language.LanguageSystem, label: 'settings.systemLanguage' },
  { value: Language.LanguageRU, label: 'settings.russian' },
  { value: Language.LanguageEN, label: 'settings.english' },
] as const

const languageOptions = computed(() =>
  LANGUAGES.map((option) => ({ value: option.value, label: t(option.label) }))
)

// The stored choice until the window has read it: the same defaults Go answers with, so a switch
// shows something true rather than nothing.
const wrap = computed(() => settings.value?.wrapLines ?? true)
const numbers = computed(() => settings.value?.lineNumbers ?? true)
const reopen = computed(() => settings.value?.reopenWorkspace ?? true)

// The space the app would come back to. Named rather than described, because the answer to "the last
// workspace" is a name, and the default space has none of its own — the interface's word is what it
// gets, exactly as it does in the title bar.
const space = ref('')
const spaceName = computed(() => space.value || t('workspaces.personal'))

const dataDir = ref('')

onMounted(async () => {
  void loadSettings()
  // Both halves of the row are cosmetic to the setting they stand beside: a failed call leaves the
  // name empty and the path blank rather than taking the row away.
  try {
    const state = await WorkspaceService.Snapshot()
    const active = state.workspaces?.find((w) => w.id === state.activeId) ?? state.workspaces?.[0]
    if (active) space.value = workspaceName(active)
  } catch {
    // Nothing to do: the sentence falls back to the interface's own word for the space.
  }
  try {
    dataDir.value = (await SystemService.StartupStatus())?.dataDir ?? ''
  } catch {
    // Nothing to do: the chip shows a dash.
  }
})
</script>

<template>
  <div class="pane">
    <SettingsSection :label="t('settings.sec.startup')">
      <SettingsRow :title="t('settings.reopenWorkspace')" :note="t('settings.reopenNote', { name: spaceName })">
        <Switch
          :model-value="reopen"
          :aria-label="t('settings.reopenWorkspace')"
          @update:model-value="setReopenWorkspace(!!$event)"
        />
      </SettingsRow>
    </SettingsSection>

    <SettingsSection :label="t('settings.sec.editor')" :note="t('settings.editorNote')">
      <SettingsRow :title="t('settings.wrapLines')">
        <Switch
          :model-value="wrap"
          :aria-label="t('settings.wrapLines')"
          @update:model-value="setEditor({ wrapLines: !!$event })"
        />
      </SettingsRow>
      <SettingsRow :title="t('settings.lineNumbers')">
        <Switch
          :model-value="numbers"
          :aria-label="t('settings.lineNumbers')"
          @update:model-value="setEditor({ lineNumbers: !!$event })"
        />
      </SettingsRow>
    </SettingsSection>

    <SettingsSection :label="t('settings.sec.language')">
      <SettingsRow :title="t('settings.interfaceLanguage')" :note="t('settings.interfaceLanguageHint')">
        <Select :model-value="language" :options="languageOptions" @update:model-value="setLanguage($event as Language)" />
      </SettingsRow>
    </SettingsSection>

    <SettingsSection :label="t('settings.sec.data')" :note="t('settings.dataNote')">
      <SettingsRow :title="t('settings.dataFolder')">
        <ValueChip :value="dataDir || '—'" mono />
        <Button size="field" @click="SystemService.OpenDataFolder()">{{ t('settings.dataOpen') }}</Button>
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
