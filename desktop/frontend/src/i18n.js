const translations = {
  en: {
    app: {
      title: "WoWCensus Companion",
      starting: "Starting…",
    },
    tabs: {
      status: "Status",
      settings: "Settings",
      activity: "Activity",
      about: "About",
    },
    banner: {
      updateAvailable: "A new version is available.",
      updateNow: "Update now",
      consent: "Share your WoWCensus data with wowcensus.io? Each new save is uploaded automatically: character names, realms and census data, never your file paths or account name.",
      turnOnUploads: "Turn on uploads",
      extrasNew: "New!",
      extras: "You can now share data from Auctionator, Journalator, TradeSkillMaster and the Dungeon Journal. Turn it on in Settings.",
      extrasOpen: "Open Settings",
      dismiss: "Dismiss",
    },
    addon: {
      cardTitle: "WoWCensus add-on",
      checking: "Checking your game folders…",
      install: "Install add-on",
      update: "Update add-on",
      installing: "Installing…",
      notFound: "No World of Warcraft installation found. If WoW is installed somewhere unusual, add its folder in Settings.",
      notInstalled: "Not installed",
      installed: "Installed {version}",
      updateAvailable: "Version {version}, update available",
      hasUpdate: "An update is available.",
      upToDate: "Up to date (version {version}) in all your game versions.",
      allInstalled: "Installed in all your game versions.",
      restartHint: "Restart World of Warcraft (or type /reload) to load it.",
    },
    saves: {
      title: "Your saves",
      looking: "Looking for your WoWCensus saves…",
      noFiles: "No WoWCensus saves found yet.",
      emptyHint: "Install the add-on above, log in to a character, and type /reload once. Your save will show up here.",
      otherTitle: "Other add-ons",
    },
    table: {
      gameVersion: "Game version",
      account: "Account",
      accountN: "Account {n}",
      size: "Size",
      lastSaved: "Last saved",
      upload: "Upload",
      addon: "Add-on",
    },
    status: {
      watching: "Watching for new saves",
      stopped: "Stopped",
      uploadsOn: "uploads on",
      uploadsOff: "uploads off",
      lastChecked: "last checked {when}",
      uploading: " · uploading…",
    },
    pill: {
      onSite: "On wowcensus.io",
      notUsed: "Not used",
      notUsedTitle: "The server could not use this save",
      uploaded: "Uploaded {when}",
      uploadedTitle: "Waiting to be added to wowcensus.io",
      retrying: "Retrying {when}",
      uploadsOff: "Uploads off",
      waiting: "Waiting…",
      nothing: "Nothing to share",
    },
    update: {
      installing: "Installing version {version}… the app will restart by itself.",
      downloading: "Downloading and verifying version {version}…",
      readyFailed: "Version {version} is ready, but the last restart failed: {error}",
      ready: "Version {version} is ready. Restart to update.",
      restartBtn: "Restart to update",
      availableFailed: "Version {version} is available, but the download failed: {error}",
      available: "Version {version} is available (you have {current}).",
      retryBtn: "Retry download",
      installingProgress: "Installing the update… the app will restart by itself.",
    },
    settings: {
      updates: "Updates",
      updatesHint: "The app checks for a new version each time it opens and once a day.",
      checkUpdate: "Check for new version",
      checkingUpdate: "Checking…",
      upToDate: "You have the latest version ({version}).",
      updateFound: "Version {version} is available. See the Status tab to update.",
      checkFailed: "Could not check for updates: {error}",
      version: "Version {version}",
      language: "Language",
      languageLabel: "Interface language",
      langAuto: "System default",
      sharing: "Sharing",
      autoUpload: "Upload new WoWCensus saves automatically",
      autoUploadHint: "Uploads include your WoWCensus.lua (character names, realms, census data). Your file paths and account folder are never sent.",
      startAtLogin: "Start when I log in to my computer (runs in the tray)",
      wowFolders: "World of Warcraft folders",
      wowFoldersHint: "Only needed if WoW is installed somewhere unusual. One folder per line.",
      checkFolders: "Check folders",
      noSavesInFolders: "No WoWCensus saves found in those folders yet.",
      saveSettings: "Save settings",
      saved: "Saved.",
      unsaved: "Unsaved changes",
      leaveTitle: "You have unsaved changes",
      leaveBody: "Save your settings before leaving?",
      leaveSave: "Save",
      leaveDiscard: "Discard",
      leaveStay: "Keep editing",
      autoClean: "Clean up WoWCensus saves automatically",
      autoCleanHint: "Removes old data wowcensus.io already has. Only runs while WoW is closed, after a backup.",
      otherAddons: "Other add-ons",
      otherAddonsHint: "Also share market and dungeon data from these add-ons. Other players' names, mail and chat text are removed on this computer before anything is sent. Only the ones you tick are shared.",
      combatLog: "Combat logs",
      combatLogHint: "Shares what mobs did in your combat logs (level, health, armor and attacks) for the bestiary on wowcensus.io. Every player name and ID is removed on this computer first. Needs Advanced Combat Logging (System > Network) and /combatlog in game. A log is sent once WoW stops writing to it.",
      cleanCombatLogs: "Delete old combat logs",
      cleanCombatLogsHint: "WoW never deletes combat logs. This removes a log once wowcensus.io has imported it and it is two weeks old, while WoW is closed. Keep it off if you upload logs to other sites.",
    },
    cleanup: {
      done: "Cleaned {before} → {after} on {when}",
      doneTitle: "Backup of the original file: {backup}",
      waitWoW: "Cleanup waits until World of Warcraft is closed",
      waitImport: "Cleanup waits until wowcensus.io has added this save",
    },
    about: {
      appTitle: "In the app",
      contributorsTitle: "Contributors",
      privacyTitle: "Privacy",
      libs: "This application is built on the following libraries: Wails V3, Go, {webview}, @wailsio/runtime, and Vite.",
      privacy: "We do not collect personal information, account details, or identifying device information. The app only transmits the data necessary for its intended functionality, using a randomized device identifier to organize uploads.",
      consent: "By enabling uploads, you acknowledge and consent to the collection, transmission, storage, and processing of game-related data for the purpose of providing and improving WoWCensus services, including generating aggregated statistics and analytics. Only data necessary for these purposes is collected. You may withdraw your consent at any time by disabling uploads.",
    },
    editions: {
      retail: "Retail",
      classic_era: "Classic Era",
      anniversary: "TBC Anniversary",
      classic: "MoP Classic",
      forever: "Forever",
      classic_beta: "Classic Beta",
      ptr: "PTR",
      xptr: "PTR",
      beta: "Beta",
    },
  },

  ko: {
    app: {
      title: "WoWCensus Companion",
      starting: "시작하는 중…",
    },
    tabs: {
      status: "상태",
      settings: "설정",
      activity: "활동",
      about: "정보",
    },
    banner: {
      updateAvailable: "새 버전이 출시되었습니다.",
      updateNow: "지금 업데이트",
      consent: "WoWCensus 데이터를 wowcensus.io와 공유하시겠습니까? 새 저장 파일이 생길 때마다 자동으로 업로드됩니다(캐릭터 이름, 서버 및 인구 조사 데이터만 포함되며 파일 경로나 계정 이름은 전송되지 않습니다).",
      turnOnUploads: "업로드 켜기",
      extrasNew: "새 기능!",
      extras: "이제 Auctionator, Journalator, TradeSkillMaster 및 던전 도감 데이터도 공유할 수 있습니다. 설정에서 켜세요.",
      extrasOpen: "설정 열기",
      dismiss: "닫기",
    },
    addon: {
      cardTitle: "WoWCensus 애드온",
      checking: "게임 폴더 확인 중…",
      install: "애드온 설치",
      update: "애드온 업데이트",
      installing: "설치 중…",
      notFound: "World of Warcraft 설치 폴더를 찾을 수 없습니다. WoW가 다른 위치에 설치되어 있다면 설정에서 폴더를 추가해 주세요.",
      notInstalled: "미설치",
      installed: "{version} 버전 설치됨",
      updateAvailable: "{version} 버전, 업데이트 가능",
      hasUpdate: "업데이트를 사용할 수 있습니다.",
      upToDate: "모든 게임 버전에서 최신 상태입니다 ({version} 버전).",
      allInstalled: "모든 게임 버전에 설치되었습니다.",
      restartHint: "적용하려면 World of Warcraft를 다시 시작하거나 게임 내에서 /reload를 입력하세요.",
    },
    saves: {
      title: "저장 데이터",
      looking: "WoWCensus 저장 파일 찾는 중…",
      noFiles: "아직 WoWCensus 저장 파일이 없습니다.",
      emptyHint: "위에서 애드온을 설치하고 캐릭터에 접속한 후 /reload를 한 번 실행하세요. 저장 파일이 여기에 표시됩니다.",
      otherTitle: "다른 애드온",
    },
    table: {
      gameVersion: "게임 버전",
      account: "계정",
      accountN: "계정 {n}",
      size: "크기",
      lastSaved: "마지막 저장",
      upload: "업로드",
      addon: "애드온",
    },
    status: {
      watching: "새 저장 파일 감시 중",
      stopped: "중지됨",
      uploadsOn: "업로드 켜짐",
      uploadsOff: "업로드 꺼짐",
      lastChecked: "마지막 확인: {when}",
      uploading: " · 업로드 중…",
    },
    pill: {
      onSite: "wowcensus.io 등록됨",
      notUsed: "미사용",
      notUsedTitle: "서버에서 이 저장 파일을 사용할 수 없습니다",
      uploaded: "업로드됨 {when}",
      uploadedTitle: "wowcensus.io에 반영 대기 중",
      retrying: "{when} 재시도 예정",
      uploadsOff: "업로드 꺼짐",
      waiting: "대기 중…",
      nothing: "공유할 내용 없음",
    },
    update: {
      installing: "{version} 버전 설치 중… 앱이 자동으로 다시 시작됩니다.",
      downloading: "{version} 버전 다운로드 및 검증 중…",
      readyFailed: "{version} 버전이 준비되었으나 지난번 다시 시작에 실패했습니다: {error}",
      ready: "{version} 버전이 준비되었습니다. 업데이트하려면 다시 시작하세요.",
      restartBtn: "다시 시작하여 업데이트",
      availableFailed: "{version} 버전을 사용할 수 있지만 다운로드에 실패했습니다: {error}",
      available: "{version} 버전을 사용할 수 있습니다 (현재 버전: {current}).",
      retryBtn: "다운로드 다시 시도",
      installingProgress: "업데이트 설치 중… 앱이 자동으로 다시 시작됩니다.",
    },
    settings: {
      updates: "업데이트",
      updatesHint: "앱을 열 때마다, 그리고 하루에 한 번 새 버전을 확인합니다.",
      checkUpdate: "새 버전 확인",
      checkingUpdate: "확인 중…",
      upToDate: "최신 버전({version})을 사용 중입니다.",
      updateFound: "{version} 버전을 사용할 수 있습니다. 상태 탭에서 업데이트하세요.",
      checkFailed: "업데이트를 확인할 수 없습니다: {error}",
      version: "버전 {version}",
      language: "언어",
      languageLabel: "인터페이스 언어",
      langAuto: "시스템 기본값",
      sharing: "공유",
      autoUpload: "새 WoWCensus 저장 파일 자동 업로드",
      autoUploadHint: "업로드에는 WoWCensus.lua 파일(캐릭터 이름, 서버, 인구 조사 데이터)이 포함됩니다. 컴퓨터의 파일 경로나 계정 폴더 이름은 전송되지 않습니다.",
      startAtLogin: "컴퓨터 부팅 시 자동 시작 (시스템 트레이에서 실행)",
      wowFolders: "World of Warcraft 폴더",
      wowFoldersHint: "WoW가 일반적이지 않은 위치에 설치된 경우에만 필요합니다. 한 줄에 하나의 폴더 경로를 입력하세요.",
      checkFolders: "폴더 확인",
      noSavesInFolders: "해당 폴더에서 아직 WoWCensus 저장 파일을 찾을 수 없습니다.",
      saveSettings: "설정 저장",
      saved: "저장되었습니다.",
      unsaved: "저장되지 않은 변경 사항",
      leaveTitle: "저장되지 않은 변경 사항이 있습니다",
      leaveBody: "나가기 전에 설정을 저장할까요?",
      leaveSave: "저장",
      leaveDiscard: "버리기",
      leaveStay: "계속 편집",
      autoClean: "WoWCensus 저장 파일 자동 정리",
      autoCleanHint: "wowcensus.io에 이미 있는 오래된 데이터를 삭제합니다. WoW가 종료된 상태에서 백업 후에만 실행됩니다.",
      otherAddons: "다른 애드온",
      otherAddonsHint: "이 애드온들의 경매장·던전 데이터도 공유합니다. 다른 플레이어의 이름, 우편과 채팅 내용은 전송 전에 이 컴퓨터에서 삭제됩니다. 선택한 애드온만 공유됩니다.",
      combatLog: "전투 기록",
      combatLogHint: "wowcensus.io 몬스터 도감을 위해 전투 기록에서 몬스터의 레벨, 생명력, 방어도, 공격 정보를 공유합니다. 모든 플레이어 이름과 ID는 먼저 이 컴퓨터에서 삭제됩니다. 고급 전투 기록(시스템 > 네트워크)과 게임 내 /combatlog가 필요합니다. WoW가 기록을 마치면 전송됩니다.",
      cleanCombatLogs: "오래된 전투 기록 삭제",
      cleanCombatLogsHint: "WoW는 전투 기록을 삭제하지 않습니다. wowcensus.io가 가져간 지 2주가 지난 기록을 WoW가 종료된 상태에서 삭제합니다. 다른 사이트에도 기록을 올린다면 끄세요.",
    },
    cleanup: {
      done: "{when}에 {before} → {after}(으)로 정리됨",
      doneTitle: "원본 파일 백업: {backup}",
      waitWoW: "World of Warcraft가 종료되면 정리합니다",
      waitImport: "wowcensus.io에 이 저장 파일이 반영되면 정리합니다",
    },
    about: {
      appTitle: "앱",
      contributorsTitle: "기여자",
      privacyTitle: "개인정보",
      libs: "이 애플리케이션은 다음 라이브러리로 만들어졌습니다: Wails V3, Go, {webview}, @wailsio/runtime, Vite.",
      privacy: "개인 정보, 계정 정보, 기기를 식별할 수 있는 정보는 수집하지 않습니다. 앱은 의도된 기능에 필요한 데이터만 전송하며, 업로드를 정리하기 위해 임의로 생성된 기기 식별자를 사용합니다.",
      consent: "업로드를 켜면 WoWCensus 서비스를 제공하고 개선하기 위해(집계 통계 및 분석 생성 포함) 게임 관련 데이터가 수집, 전송, 저장 및 처리되는 것을 인지하고 이에 동의하게 됩니다. 이 목적에 필요한 데이터만 수집합니다. 업로드를 끄면 언제든지 동의를 철회할 수 있습니다.",
    },
    editions: {
      retail: "리테일",
      classic_era: "클래식 시대",
      anniversary: "불성 애니버서리",
      classic: "판다리아 클래식",
      forever: "Forever",
      classic_beta: "클래식 베타",
      ptr: "PTR",
      xptr: "PTR",
      beta: "베타",
    },
  },

  es: {
    app: {
      title: "WoWCensus Companion",
      starting: "Iniciando…",
    },
    tabs: {
      status: "Estado",
      settings: "Ajustes",
      activity: "Actividad",
      about: "Acerca de",
    },
    banner: {
      updateAvailable: "Hay una nueva versión disponible.",
      updateNow: "Actualizar ahora",
      consent: "¿Compartir tus datos de WoWCensus con wowcensus.io? Cada nuevo guardado se sube automáticamente: nombres de personajes, reinos y datos del censo, nunca las rutas de tus archivos ni el nombre de tu cuenta.",
      turnOnUploads: "Activar subidas",
      extrasNew: "¡Nuevo!",
      extras: "Ahora puedes compartir datos de Auctionator, Journalator, TradeSkillMaster y la Guía de mazmorras. Actívalo en Ajustes.",
      extrasOpen: "Abrir ajustes",
      dismiss: "Descartar",
    },
    addon: {
      cardTitle: "Add-on WoWCensus",
      checking: "Comprobando las carpetas del juego…",
      install: "Instalar add-on",
      update: "Actualizar add-on",
      installing: "Instalando…",
      notFound: "No se encontró ninguna instalación de World of Warcraft. Si WoW está instalado en una ubicación personalizada, añade su carpeta en Ajustes.",
      notInstalled: "No instalado",
      installed: "Instalado {version}",
      updateAvailable: "Versión {version}, actualización disponible",
      hasUpdate: "Hay una actualización disponible.",
      upToDate: "Al día (versión {version}) en todas tus versiones del juego.",
      allInstalled: "Instalado en todas tus versiones del juego.",
      restartHint: "Reinicia World of Warcraft (o escribe /reload) para cargarlo.",
    },
    saves: {
      title: "Tus guardados",
      looking: "Buscando tus guardados de WoWCensus…",
      noFiles: "Aún no se han encontrado guardados de WoWCensus.",
      emptyHint: "Instala el add-on arriba, inicia sesión con un personaje y escribe /reload una vez. Tu guardado aparecerá aquí.",
      otherTitle: "Otros addons",
    },
    table: {
      gameVersion: "Versión del juego",
      account: "Cuenta",
      accountN: "Cuenta {n}",
      size: "Tamaño",
      lastSaved: "Último guardado",
      upload: "Subida",
      addon: "Addon",
    },
    status: {
      watching: "Buscando nuevos guardados",
      stopped: "Detenido",
      uploadsOn: "subidas activadas",
      uploadsOff: "subidas desactivadas",
      lastChecked: "última comprobación {when}",
      uploading: " · subiendo…",
    },
    pill: {
      onSite: "En wowcensus.io",
      notUsed: "No usado",
      notUsedTitle: "El servidor no pudo usar este guardado",
      uploaded: "Subido {when}",
      uploadedTitle: "Esperando a ser añadido a wowcensus.io",
      retrying: "Reintentando {when}",
      uploadsOff: "Subidas desactivadas",
      waiting: "Esperando…",
      nothing: "Nada que compartir",
    },
    update: {
      installing: "Instalando la versión {version}… la aplicación se reiniciará automáticamente.",
      downloading: "Descargando y verificando la versión {version}…",
      readyFailed: "La versión {version} está lista, pero el último reinicio falló: {error}",
      ready: "La versión {version} está lista. Reinicia para actualizar.",
      restartBtn: "Reiniciar para actualizar",
      availableFailed: "La versión {version} está disponible, pero la descarga falló: {error}",
      available: "La versión {version} está disponible (tienes la {current}).",
      retryBtn: "Reintentar descarga",
      installingProgress: "Instalando la actualización… la aplicación se reiniciará automáticamente.",
    },
    settings: {
      updates: "Actualizaciones",
      updatesHint: "La aplicación busca una versión nueva cada vez que se abre y una vez al día.",
      checkUpdate: "Buscar nueva versión",
      checkingUpdate: "Comprobando…",
      upToDate: "Tienes la última versión ({version}).",
      updateFound: "La versión {version} está disponible. Actualiza desde la pestaña Estado.",
      checkFailed: "No se pudo comprobar si hay actualizaciones: {error}",
      version: "Versión {version}",
      language: "Idioma",
      languageLabel: "Idioma de la interfaz",
      langAuto: "Predeterminado del sistema",
      sharing: "Compartir",
      autoUpload: "Subir nuevos guardados de WoWCensus automáticamente",
      autoUploadHint: "Las subidas incluyen tu archivo WoWCensus.lua (nombres de personajes, reinos, datos del censo). Las rutas de tus archivos y tu carpeta de cuenta nunca se envían.",
      startAtLogin: "Iniciar al encender el ordenador (se ejecuta en la bandeja)",
      wowFolders: "Carpetas de World of Warcraft",
      wowFoldersHint: "Solo necesario si WoW está instalado en una ubicación personalizada. Una carpeta por línea.",
      checkFolders: "Comprobar carpetas",
      noSavesInFolders: "Aún no se han encontrado guardados de WoWCensus en esas carpetas.",
      saveSettings: "Guardar ajustes",
      saved: "Guardado.",
      unsaved: "Cambios sin guardar",
      leaveTitle: "Tienes cambios sin guardar",
      leaveBody: "¿Guardar los ajustes antes de salir?",
      leaveSave: "Guardar",
      leaveDiscard: "Descartar",
      leaveStay: "Seguir editando",
      autoClean: "Limpiar automáticamente los guardados de WoWCensus",
      autoCleanHint: "Elimina datos antiguos que wowcensus.io ya tiene. Solo con WoW cerrado y tras una copia de seguridad.",
      otherAddons: "Otros addons",
      otherAddonsHint: "Comparte también los datos de mercado y de mazmorras de estos addons. Los nombres de otros jugadores y el texto del correo y del chat se eliminan en este equipo antes de enviar nada. Solo se comparten los que marques.",
      combatLog: "Registros de combate",
      combatLogHint: "Comparte lo que hicieron los enemigos en tus registros de combate (nivel, salud, armadura y ataques) para el bestiario de wowcensus.io. Todos los nombres e ID de jugadores se eliminan antes en este equipo. Requiere el registro de combate avanzado (Sistema > Red) y /combatlog en el juego. Cada registro se envía cuando WoW deja de escribir en él.",
      cleanCombatLogs: "Eliminar registros de combate antiguos",
      cleanCombatLogsHint: "WoW nunca borra los registros de combate. Esto elimina un registro cuando wowcensus.io ya lo ha importado y tiene dos semanas, con WoW cerrado. Déjalo desactivado si subes registros a otros sitios.",
    },
    cleanup: {
      done: "Reducido de {before} a {after} el {when}",
      doneTitle: "Copia de seguridad del archivo original: {backup}",
      waitWoW: "La limpieza espera a que cierres World of Warcraft",
      waitImport: "La limpieza espera a que wowcensus.io añada este guardado",
    },
    about: {
      appTitle: "En la aplicación",
      contributorsTitle: "Colaboradores",
      privacyTitle: "Privacidad",
      libs: "Esta aplicación está construida con las siguientes bibliotecas: Wails V3, Go, {webview}, @wailsio/runtime y Vite.",
      privacy: "No recopilamos información personal, datos de la cuenta ni información que identifique tu dispositivo. La aplicación solo transmite los datos necesarios para su funcionamiento previsto y usa un identificador de dispositivo aleatorio para organizar las subidas.",
      consent: "Al activar las subidas, reconoces y aceptas la recopilación, transmisión, almacenamiento y tratamiento de datos relacionados con el juego para prestar y mejorar los servicios de WoWCensus, incluida la generación de estadísticas y análisis agregados. Solo se recopilan los datos necesarios para estos fines. Puedes retirar tu consentimiento en cualquier momento desactivando las subidas.",
    },
    editions: {
      retail: "Retail",
      classic_era: "Era clásica",
      anniversary: "TBC Aniversario",
      classic: "MoP Classic",
      forever: "Forever",
      classic_beta: "Beta de Classic",
      ptr: "RPP",
      xptr: "RPP",
      beta: "Beta",
    },
  },

  de: {
    app: {
      title: "WoWCensus Companion",
      starting: "Wird gestartet…",
    },
    tabs: {
      status: "Status",
      settings: "Einstellungen",
      activity: "Aktivität",
      about: "Info",
    },
    banner: {
      updateAvailable: "Eine neue Version ist verfügbar.",
      updateNow: "Jetzt aktualisieren",
      consent: "WoWCensus-Daten mit wowcensus.io teilen? Jeder neue Speicherstand wird automatisch hochgeladen: Charakternamen, Realms und Zensusdaten, niemals Dateipfade oder Accountname.",
      turnOnUploads: "Uploads aktivieren",
      extrasNew: "Neu!",
      extras: "Du kannst jetzt auch Daten aus Auctionator, Journalator, TradeSkillMaster und dem Dungeonkompendium teilen. Aktiviere es in den Einstellungen.",
      extrasOpen: "Einstellungen öffnen",
      dismiss: "Ausblenden",
    },
    addon: {
      cardTitle: "WoWCensus Add-on",
      checking: "Spielordner werden überprüft…",
      install: "Add-on installieren",
      update: "Add-on aktualisieren",
      installing: "Wird installiert…",
      notFound: "Keine World of Warcraft-Installation gefunden. Falls WoW an einem ungewöhnlichen Ort liegt, Ordner in den Einstellungen hinzufügen.",
      notInstalled: "Nicht installiert",
      installed: "Installiert {version}",
      updateAvailable: "Version {version}, Update verfügbar",
      hasUpdate: "Ein Update ist verfügbar.",
      upToDate: "Auf dem neuesten Stand (Version {version}) in allen deinen Spielversionen.",
      allInstalled: "In allen deinen Spielversionen installiert.",
      restartHint: "Starte World of Warcraft neu (oder gib /reload ein), um es zu laden.",
    },
    saves: {
      title: "Deine Speicherstände",
      looking: "Suche nach WoWCensus-Speicherständen…",
      noFiles: "Noch keine WoWCensus-Speicherstände gefunden.",
      emptyHint: "Installiere das Add-on oben, logge dich mit einem Charakter ein und tippe einmal /reload ein. Dein Speicherstand erscheint hier.",
      otherTitle: "Andere Add-ons",
    },
    table: {
      gameVersion: "Spielversion",
      account: "Account",
      accountN: "Account {n}",
      size: "Größe",
      lastSaved: "Zuletzt gespeichert",
      upload: "Upload",
      addon: "Add-on",
    },
    status: {
      watching: "Wartet auf neue Speicherstände",
      stopped: "Angehalten",
      uploadsOn: "Uploads an",
      uploadsOff: "Uploads aus",
      lastChecked: "zuletzt geprüft: {when}",
      uploading: " · wird hochgeladen…",
    },
    pill: {
      onSite: "Auf wowcensus.io",
      notUsed: "Nicht verwendet",
      notUsedTitle: "Der Server konnte diesen Speicherstand nicht verwenden",
      uploaded: "Hochgeladen {when}",
      uploadedTitle: "Wartet auf Aufnahme in wowcensus.io",
      retrying: "Wiederholung am {when}",
      uploadsOff: "Uploads aus",
      waiting: "Warten…",
      nothing: "Nichts zu teilen",
    },
    update: {
      installing: "Version {version} wird installiert… Die App startet automatisch neu.",
      downloading: "Version {version} wird heruntergeladen und überprüft…",
      readyFailed: "Version {version} ist bereit, aber der letzte Neustart ist fehlgeschlagen: {error}",
      ready: "Version {version} ist bereit. Zum Aktualisieren neu starten.",
      restartBtn: "Neu starten zum Aktualisieren",
      availableFailed: "Version {version} ist verfügbar, aber der Download ist fehlgeschlagen: {error}",
      available: "Version {version} ist verfügbar (installiert: {current}).",
      retryBtn: "Download wiederholen",
      installingProgress: "Update wird installiert… Die App startet automatisch neu.",
    },
    settings: {
      updates: "Aktualisierungen",
      updatesHint: "Die App sucht bei jedem Öffnen und einmal täglich nach einer neuen Version.",
      checkUpdate: "Nach neuer Version suchen",
      checkingUpdate: "Wird geprüft…",
      upToDate: "Du hast die neueste Version ({version}).",
      updateFound: "Version {version} ist verfügbar. Aktualisiere im Tab „Status“.",
      checkFailed: "Suche nach Aktualisierungen fehlgeschlagen: {error}",
      version: "Version {version}",
      language: "Sprache",
      languageLabel: "Oberflächensprache",
      langAuto: "Systemstandard",
      sharing: "Freigabe",
      autoUpload: "Neue WoWCensus-Speicherstände automatisch hochladen",
      autoUploadHint: "Uploads enthalten deine WoWCensus.lua (Charakternamen, Realms, Zensusdaten). Dateipfade und Accountordner werden niemals übertragen.",
      startAtLogin: "Beim Anmelden am Computer starten (im Infobereich ausführen)",
      wowFolders: "World of Warcraft-Ordner",
      wowFoldersHint: "Nur erforderlich, wenn WoW an einem ungewöhnlichen Ort installiert ist. Ein Ordner pro Zeile.",
      checkFolders: "Ordner überprüfen",
      noSavesInFolders: "In diesen Ordnern wurden noch keine WoWCensus-Speicherstände gefunden.",
      saveSettings: "Einstellungen speichern",
      saved: "Gespeichert.",
      unsaved: "Nicht gespeicherte Änderungen",
      leaveTitle: "Du hast nicht gespeicherte Änderungen",
      leaveBody: "Einstellungen vor dem Verlassen speichern?",
      leaveSave: "Speichern",
      leaveDiscard: "Verwerfen",
      leaveStay: "Weiter bearbeiten",
      autoClean: "WoWCensus-Speicherdateien automatisch aufräumen",
      autoCleanHint: "Entfernt alte Daten, die wowcensus.io schon hat. Nur bei geschlossenem WoW und nach einer Sicherung.",
      otherAddons: "Andere Add-ons",
      otherAddonsHint: "Teilt auch Markt- und Dungeondaten dieser Add-ons. Namen anderer Spieler sowie Post- und Chattexte werden vor dem Senden auf diesem Computer entfernt. Geteilt wird nur, was du ankreuzt.",
      combatLog: "Kampflogs",
      combatLogHint: "Teilt, was Gegner in deinen Kampflogs getan haben (Stufe, Gesundheit, Rüstung und Angriffe), für das Bestiarium auf wowcensus.io. Alle Spielernamen und IDs werden vorher auf diesem Computer entfernt. Benötigt erweiterte Kampfaufzeichnung (System > Netzwerk) und /combatlog im Spiel. Ein Log wird gesendet, sobald WoW nicht mehr hineinschreibt.",
      cleanCombatLogs: "Alte Kampflogs löschen",
      cleanCombatLogsHint: "WoW löscht Kampflogs nie. Dies entfernt ein Log, sobald wowcensus.io es importiert hat und es zwei Wochen alt ist, bei geschlossenem WoW. Lass es aus, wenn du Logs auch auf andere Seiten hochlädst.",
    },
    cleanup: {
      done: "Am {when} von {before} auf {after} verkleinert",
      doneTitle: "Sicherung der Originaldatei: {backup}",
      waitWoW: "Das Aufräumen wartet, bis World of Warcraft geschlossen ist",
      waitImport: "Das Aufräumen wartet, bis wowcensus.io diese Speicherung übernommen hat",
    },
    about: {
      appTitle: "In der App",
      contributorsTitle: "Mitwirkende",
      privacyTitle: "Datenschutz",
      libs: "Diese Anwendung basiert auf den folgenden Bibliotheken: Wails V3, Go, {webview}, @wailsio/runtime und Vite.",
      privacy: "Wir erheben keine personenbezogenen Daten, keine Accountdaten und keine Informationen, die dein Gerät identifizieren. Die App überträgt nur die für ihre Funktion nötigen Daten und verwendet eine zufällige Gerätekennung, um Uploads zuzuordnen.",
      consent: "Wenn du Uploads aktivierst, nimmst du zur Kenntnis und willigst ein, dass spielbezogene Daten erhoben, übertragen, gespeichert und verarbeitet werden, um die Dienste von WoWCensus bereitzustellen und zu verbessern, einschließlich zusammengefasster Statistiken und Auswertungen. Es werden nur die dafür nötigen Daten erhoben. Du kannst deine Einwilligung jederzeit widerrufen, indem du Uploads deaktivierst.",
    },
    editions: {
      retail: "Retail",
      classic_era: "Classic Era",
      anniversary: "TBC Jubiläum",
      classic: "MoP Classic",
      forever: "Forever",
      classic_beta: "Classic Beta",
      ptr: "PTR",
      xptr: "PTR",
      beta: "Beta",
    },
  },

  fr: {
    app: {
      title: "WoWCensus Companion",
      starting: "Démarrage…",
    },
    tabs: {
      status: "État",
      settings: "Paramètres",
      activity: "Activité",
      about: "À propos",
    },
    banner: {
      updateAvailable: "Une nouvelle version est disponible.",
      updateNow: "Mettre à jour maintenant",
      consent: "Partager vos données WoWCensus avec wowcensus.io ? Chaque nouvelle sauvegarde est envoyée automatiquement : noms des personnages, royaumes et données de recensement, jamais vos chemins de fichiers ni le nom de votre compte.",
      turnOnUploads: "Activer les téléversements",
      extrasNew: "Nouveau !",
      extras: "Vous pouvez désormais partager les données d'Auctionator, Journalator, TradeSkillMaster et du Journal des donjons. Activez-le dans les Paramètres.",
      extrasOpen: "Ouvrir les paramètres",
      dismiss: "Ignorer",
    },
    addon: {
      cardTitle: "Add-on WoWCensus",
      checking: "Vérification de vos dossiers de jeu…",
      install: "Installer l'add-on",
      update: "Mettre à jour l'add-on",
      installing: "Installation…",
      notFound: "Aucune installation de World of Warcraft trouvée. Si WoW est installé dans un emplacement personnalisé, ajoutez son dossier dans les Paramètres.",
      notInstalled: "Non installé",
      installed: "Installé {version}",
      updateAvailable: "Version {version}, mise à jour disponible",
      hasUpdate: "Une mise à jour est disponible.",
      upToDate: "À jour (version {version}) dans toutes vos versions du jeu.",
      allInstalled: "Installé dans toutes vos versions du jeu.",
      restartHint: "Redémarrez World of Warcraft (ou tapez /reload) pour le charger.",
    },
    saves: {
      title: "Vos sauvegardes",
      looking: "Recherche de vos sauvegardes WoWCensus…",
      noFiles: "Aucune sauvegarde WoWCensus trouvée pour le moment.",
      emptyHint: "Installez l'add-on ci-dessus, connectez-vous avec un personnage et tapez /reload une fois. Votre sauvegarde apparaîtra ici.",
      otherTitle: "Autres addons",
    },
    table: {
      gameVersion: "Version du jeu",
      account: "Compte",
      accountN: "Compte {n}",
      size: "Taille",
      lastSaved: "Dernière sauvegarde",
      upload: "Téléversement",
      addon: "Addon",
    },
    status: {
      watching: "Surveillance des nouvelles sauvegardes",
      stopped: "Arrêté",
      uploadsOn: "téléversements activés",
      uploadsOff: "téléversements désactivés",
      lastChecked: "dernière vérification {when}",
      uploading: " · téléversement…",
    },
    pill: {
      onSite: "Sur wowcensus.io",
      notUsed: "Non utilisé",
      notUsedTitle: "Le serveur n'a pas pu utiliser cette sauvegarde",
      uploaded: "Téléversé le {when}",
      uploadedTitle: "En attente d'ajout sur wowcensus.io",
      retrying: "Nouvel essai le {when}",
      uploadsOff: "Téléversements désactivés",
      waiting: "En attente…",
      nothing: "Rien à partager",
    },
    update: {
      installing: "Installation de la version {version}… l'application va redémarrer automatiquement.",
      downloading: "Téléchargement et vérification de la version {version}…",
      readyFailed: "La version {version} est prête, mais le dernier redémarrage a échoué : {error}",
      ready: "La version {version} est prête. Redémarrez pour mettre à jour.",
      restartBtn: "Redémarrer pour mettre à jour",
      availableFailed: "La version {version} est disponible, mais le téléchargement a échoué : {error}",
      available: "La version {version} est disponible (vous avez la {current}).",
      retryBtn: "Réessayer le téléchargement",
      installingProgress: "Installation de la mise à jour… l'application va redémarrer automatiquement.",
    },
    settings: {
      updates: "Mises à jour",
      updatesHint: "L'application recherche une nouvelle version à chaque ouverture et une fois par jour.",
      checkUpdate: "Rechercher une nouvelle version",
      checkingUpdate: "Vérification…",
      upToDate: "Vous avez la dernière version ({version}).",
      updateFound: "La version {version} est disponible. Mettez à jour depuis l'onglet État.",
      checkFailed: "Impossible de rechercher les mises à jour : {error}",
      version: "Version {version}",
      language: "Langue",
      languageLabel: "Langue de l'interface",
      langAuto: "Par défaut du système",
      sharing: "Partage",
      autoUpload: "Téléverser automatiquement les nouvelles sauvegardes WoWCensus",
      autoUploadHint: "Les téléversements incluent votre fichier WoWCensus.lua (noms de personnages, royaumes, données de recensement). Vos chemins de fichiers et dossiers de compte ne sont jamais envoyés.",
      startAtLogin: "Lancer au démarrage de la session (s'exécute dans la barre des tâches)",
      wowFolders: "Dossiers World of Warcraft",
      wowFoldersHint: "Nécessaire uniquement si WoW est installé dans un emplacement inhabituel. Un dossier par ligne.",
      checkFolders: "Vérifier les dossiers",
      noSavesInFolders: "Aucune sauvegarde WoWCensus trouvée dans ces dossiers pour l'instant.",
      saveSettings: "Enregistrer les paramètres",
      saved: "Enregistré.",
      unsaved: "Modifications non enregistrées",
      leaveTitle: "Vous avez des modifications non enregistrées",
      leaveBody: "Enregistrer vos paramètres avant de quitter ?",
      leaveSave: "Enregistrer",
      leaveDiscard: "Ignorer",
      leaveStay: "Continuer",
      autoClean: "Nettoyer automatiquement les sauvegardes WoWCensus",
      autoCleanHint: "Supprime les anciennes données que wowcensus.io possède déjà. Seulement quand WoW est fermé, après une copie de sauvegarde.",
      otherAddons: "Autres addons",
      otherAddonsHint: "Partage aussi les données de marché et de donjons de ces addons. Les noms des autres joueurs et le texte du courrier et du chat sont retirés sur cet ordinateur avant tout envoi. Seuls les addons cochés sont partagés.",
      combatLog: "Journaux de combat",
      combatLogHint: "Partage ce que les monstres ont fait dans vos journaux de combat (niveau, vie, armure et attaques) pour le bestiaire de wowcensus.io. Tous les noms et identifiants de joueurs sont d'abord supprimés sur cet ordinateur. Nécessite la journalisation avancée du combat (Système > Réseau) et /combatlog en jeu. Un journal est envoyé quand WoW a fini d'y écrire.",
      cleanCombatLogs: "Supprimer les anciens journaux de combat",
      cleanCombatLogsHint: "WoW ne supprime jamais les journaux de combat. Ceci supprime un journal une fois importé par wowcensus.io et vieux de deux semaines, quand WoW est fermé. Laissez désactivé si vous envoyez vos journaux sur d'autres sites.",
    },
    cleanup: {
      done: "Réduit de {before} à {after} le {when}",
      doneTitle: "Copie du fichier d'origine : {backup}",
      waitWoW: "Le nettoyage attend la fermeture de World of Warcraft",
      waitImport: "Le nettoyage attend que wowcensus.io ait ajouté cette sauvegarde",
    },
    about: {
      appTitle: "Dans l'application",
      contributorsTitle: "Contributeurs",
      privacyTitle: "Confidentialité",
      libs: "Cette application repose sur les bibliothèques suivantes : Wails V3, Go, {webview}, @wailsio/runtime et Vite.",
      privacy: "Nous ne collectons aucune information personnelle, aucune donnée de compte ni aucune information permettant d'identifier votre appareil. L'application ne transmet que les données nécessaires à son fonctionnement et utilise un identifiant d'appareil aléatoire pour organiser les envois.",
      consent: "En activant les envois, vous reconnaissez et acceptez la collecte, la transmission, le stockage et le traitement de données liées au jeu afin de fournir et d'améliorer les services WoWCensus, y compris la production de statistiques et d'analyses agrégées. Seules les données nécessaires à ces fins sont collectées. Vous pouvez retirer votre consentement à tout moment en désactivant les envois.",
    },
    editions: {
      retail: "Retail",
      classic_era: "Classic Era",
      anniversary: "Anniversaire TBC",
      classic: "MoP Classic",
      forever: "Forever",
      classic_beta: "Bêta Classic",
      ptr: "RPT",
      xptr: "RPT",
      beta: "Bêta",
    },
  },

  it: {
    app: {
      title: "WoWCensus Companion",
      starting: "Avvio in corso…",
    },
    tabs: {
      status: "Stato",
      settings: "Impostazioni",
      activity: "Attività",
      about: "Informazioni",
    },
    banner: {
      updateAvailable: "È disponibile una nuova versione.",
      updateNow: "Aggiorna ora",
      consent: "Vuoi condividere i tuoi dati WoWCensus con wowcensus.io? Ogni nuovo salvataggio viene caricato automaticamente: nomi dei personaggi, reami e dati del censimento, mai i percorsi dei file o il nome del tuo account.",
      turnOnUploads: "Attiva caricamenti",
      extrasNew: "Novità!",
      extras: "Ora puoi condividere anche i dati di Auctionator, Journalator, TradeSkillMaster e del Diario delle spedizioni. Attivalo nelle Impostazioni.",
      extrasOpen: "Apri impostazioni",
      dismiss: "Ignora",
    },
    addon: {
      cardTitle: "Add-on WoWCensus",
      checking: "Controllo delle tue cartelle di gioco…",
      install: "Installa add-on",
      update: "Aggiorna add-on",
      installing: "Installazione in corso…",
      notFound: "Nessuna installazione di World of Warcraft trovata. Se WoW è installato in una posizione insolita, aggiungi la sua cartella nelle Impostazioni.",
      notInstalled: "Non installato",
      installed: "Installato {version}",
      updateAvailable: "Versione {version}, aggiornamento disponibile",
      hasUpdate: "È disponibile un aggiornamento.",
      upToDate: "Aggiornato (versione {version}) in tutte le tue versioni di gioco.",
      allInstalled: "Installato in tutte le tue versioni di gioco.",
      restartHint: "Riavvia World of Warcraft (o digita /reload) per caricarlo.",
    },
    saves: {
      title: "I tuoi salvataggi",
      looking: "Ricerca dei tuoi salvataggi WoWCensus in corso…",
      noFiles: "Nessun salvataggio di WoWCensus ancora trovato.",
      emptyHint: "Installa l'add-on sopra, accedi con un personaggio e digita /reload una volta. Il tuo salvataggio apparirà qui.",
      otherTitle: "Altri addon",
    },
    table: {
      gameVersion: "Versione di gioco",
      account: "Account",
      accountN: "Account {n}",
      size: "Dimensione",
      lastSaved: "Ultimo salvataggio",
      upload: "Caricamento",
      addon: "Addon",
    },
    status: {
      watching: "Monitoraggio nuovi salvataggi",
      stopped: "Fermato",
      uploadsOn: "caricamenti attivi",
      uploadsOff: "caricamenti disattivati",
      lastChecked: "ultimo controllo {when}",
      uploading: " · caricamento in corso…",
    },
    pill: {
      onSite: "Su wowcensus.io",
      notUsed: "Non usato",
      notUsedTitle: "Il server non ha potuto usare questo salvataggio",
      uploaded: "Caricato il {when}",
      uploadedTitle: "In attesa di aggiunta su wowcensus.io",
      retrying: "Nuovo tentativo il {when}",
      uploadsOff: "Caricamenti disattivati",
      waiting: "In attesa…",
      nothing: "Niente da condividere",
    },
    update: {
      installing: "Installazione della versione {version} in corso… l'app si riavvierà automaticamente.",
      downloading: "Download e verifica della versione {version} in corso…",
      readyFailed: "La versione {version} è pronta, ma l'ultimo riavvio non è riuscito: {error}",
      ready: "La versione {version} è pronta. Riavvia per aggiornare.",
      restartBtn: "Riavvia per aggiornare",
      availableFailed: "La versione {version} è disponibile, ma il download non è riuscito: {error}",
      available: "La versione {version} è disponibile (versione attuale: {current}).",
      retryBtn: "Riprova il download",
      installingProgress: "Installazione dell'aggiornamento in corso… l'app si riavvierà automaticamente.",
    },
    settings: {
      updates: "Aggiornamenti",
      updatesHint: "L'app cerca una nuova versione a ogni apertura e una volta al giorno.",
      checkUpdate: "Cerca nuova versione",
      checkingUpdate: "Verifica in corso…",
      upToDate: "Hai l'ultima versione ({version}).",
      updateFound: "La versione {version} è disponibile. Aggiorna dalla scheda Stato.",
      checkFailed: "Impossibile cercare aggiornamenti: {error}",
      version: "Versione {version}",
      language: "Lingua",
      languageLabel: "Lingua dell'interfaccia",
      langAuto: "Predefinito di sistema",
      sharing: "Condivisione",
      autoUpload: "Carica automaticamente i nuovi salvataggi di WoWCensus",
      autoUploadHint: "I caricamenti includono il file WoWCensus.lua (nomi dei personaggi, reami, dati del censimento). I percorsi dei file e la cartella dell'account non vengono mai inviati.",
      startAtLogin: "Avvia all'accesso al computer (in esecuzione nella barra delle applicazioni)",
      wowFolders: "Cartelle di World of Warcraft",
      wowFoldersHint: "Necessario solo se WoW è installato in una posizione insolita. Una cartella per riga.",
      checkFolders: "Verifica cartelle",
      noSavesInFolders: "Nessun salvataggio di WoWCensus ancora trovato in quelle cartelle.",
      saveSettings: "Salva impostazioni",
      saved: "Salvato.",
      unsaved: "Modifiche non salvate",
      leaveTitle: "Hai modifiche non salvate",
      leaveBody: "Salvare le impostazioni prima di uscire?",
      leaveSave: "Salva",
      leaveDiscard: "Scarta",
      leaveStay: "Continua a modificare",
      autoClean: "Pulisci automaticamente i salvataggi di WoWCensus",
      autoCleanHint: "Rimuove i dati vecchi che wowcensus.io ha già. Solo a WoW chiuso, dopo una copia di sicurezza.",
      otherAddons: "Altri addon",
      otherAddonsHint: "Condivide anche i dati di mercato e dei dungeon di questi addon. I nomi degli altri giocatori e i testi di posta e chat vengono rimossi su questo computer prima dell'invio. Vengono condivisi solo quelli che selezioni.",
      combatLog: "Registri di combattimento",
      combatLogHint: "Condivide ciò che i mostri hanno fatto nei tuoi registri di combattimento (livello, salute, armatura e attacchi) per il bestiario di wowcensus.io. Ogni nome e ID dei giocatori viene rimosso prima su questo computer. Richiede la registrazione avanzata del combattimento (Sistema > Rete) e /combatlog nel gioco. Un registro viene inviato quando WoW smette di scriverci.",
      cleanCombatLogs: "Elimina i vecchi registri di combattimento",
      cleanCombatLogsHint: "WoW non elimina mai i registri di combattimento. Questa opzione rimuove un registro quando wowcensus.io lo ha importato e ha due settimane, a WoW chiuso. Lasciala disattivata se carichi i registri anche su altri siti.",
    },
    cleanup: {
      done: "Ridotto da {before} a {after} il {when}",
      doneTitle: "Copia del file originale: {backup}",
      waitWoW: "La pulizia attende la chiusura di World of Warcraft",
      waitImport: "La pulizia attende che wowcensus.io aggiunga questo salvataggio",
    },
    about: {
      appTitle: "Nell'app",
      contributorsTitle: "Collaboratori",
      privacyTitle: "Privacy",
      libs: "Questa applicazione è basata sulle seguenti librerie: Wails V3, Go, {webview}, @wailsio/runtime e Vite.",
      privacy: "Non raccogliamo informazioni personali, dati dell'account né informazioni che identificano il tuo dispositivo. L'app trasmette solo i dati necessari al suo funzionamento e usa un identificatore di dispositivo casuale per organizzare i caricamenti.",
      consent: "Attivando i caricamenti, prendi atto e acconsenti alla raccolta, trasmissione, conservazione ed elaborazione dei dati di gioco allo scopo di fornire e migliorare i servizi WoWCensus, inclusa la produzione di statistiche e analisi aggregate. Vengono raccolti solo i dati necessari a questi scopi. Puoi revocare il consenso in qualsiasi momento disattivando i caricamenti.",
    },
    editions: {
      retail: "Retail",
      classic_era: "Classic Era",
      anniversary: "Anniversario TBC",
      classic: "MoP Classic",
      forever: "Forever",
      classic_beta: "Beta Classic",
      ptr: "PTR",
      xptr: "PTR",
      beta: "Beta",
    },
  },
};

const STORAGE_KEY = "wowcensus_lang";
let activeLanguage = "en";

export function detectBrowserLanguage() {
  const navLang = (navigator.language || (navigator.languages && navigator.languages[0]) || "").toLowerCase();
  for (const code of ["ko", "es", "de", "fr", "it"]) {
    if (navLang.startsWith(code)) return code;
  }
  return "en";
}

export function getStoredLanguage() {
  try {
    return localStorage.getItem(STORAGE_KEY) || "";
  } catch {
    return "";
  }
}

export function getLanguage() {
  return activeLanguage;
}

export function setLanguage(lang) {
  let target = lang;
  if (!target || target === "auto") {
    target = detectBrowserLanguage();
  }
  if (!translations[target]) {
    target = "en";
  }
  activeLanguage = target;
  try {
    if (lang) localStorage.setItem(STORAGE_KEY, lang);
  } catch {}
  document.documentElement.lang = target;
  applyTranslations();
}

function resolveKey(obj, path) {
  return path.split(".").reduce((acc, part) => (acc && acc[part] !== undefined ? acc[part] : undefined), obj);
}

export function t(key, params = {}) {
  const dict = translations[activeLanguage] || translations.en;
  let val = resolveKey(dict, key);
  if (val === undefined) {
    val = resolveKey(translations.en, key);
  }
  if (val === undefined) return key;
  return String(val).replace(/\{(\w+)\}/g, (_, k) => (params[k] !== undefined ? params[k] : `{${k}}`));
}

export function formatMissingAddon(missing, total) {
  const lang = activeLanguage;
  if (lang === "ko") {
    return `${total}개의 게임 버전 중 ${missing}개에 설치되지 않았습니다.`;
  }
  if (lang === "es") {
    const vWord = total === 1 ? "versión" : "versiones";
    return `No instalado en ${missing} de tus ${total} ${vWord} del juego.`;
  }
  if (lang === "de") {
    const vWord = total === 1 ? "Spielversion" : "Spielversionen";
    return `Für ${missing} deiner ${total} ${vWord} nicht installiert.`;
  }
  if (lang === "fr") {
    const vWord = total === 1 ? "version" : "versions";
    return `Non installé pour ${missing} de vos ${total} ${vWord} du jeu.`;
  }
  if (lang === "it") {
    const vWord = total === 1 ? "versione" : "versioni";
    return `Non installato per ${missing} delle tue ${total} ${vWord} di gioco.`;
  }
  const vWord = total === 1 ? "game version" : "game versions";
  return `Not installed for ${missing} of your ${total} ${vWord}.`;
}

export function formatFoundSaves(count, versions) {
  const lang = activeLanguage;
  if (lang === "ko") {
    return `${versions}개 게임 버전에서 ${count}개의 WoWCensus 저장 파일을 찾았습니다.`;
  }
  if (lang === "es") {
    const sWord = count === 1 ? "guardado" : "guardados";
    const vWord = versions === 1 ? "versión" : "versiones";
    const verb = count === 1 ? "Se encontró" : "Se encontraron";
    return `${verb} ${count} ${sWord} de WoWCensus en ${versions} ${vWord} del juego.`;
  }
  if (lang === "de") {
    const sWord = count === 1 ? "WoWCensus-Speicherstand" : "WoWCensus-Speicherstände";
    const vWord = versions === 1 ? "Spielversion" : "Spielversionen";
    return `${count} ${sWord} in ${versions} ${vWord} gefunden.`;
  }
  if (lang === "fr") {
    const sWord = count === 1 ? "sauvegarde" : "sauvegardes";
    const vWord = versions === 1 ? "version" : "versions";
    const tWord = count === 1 ? "trouvée" : "trouvées";
    return `${count} ${sWord} WoWCensus ${tWord} dans ${versions} ${vWord} du jeu.`;
  }
  if (lang === "it") {
    const sWord = count === 1 ? "salvataggio" : "salvataggi";
    const vWord = versions === 1 ? "versione" : "versioni";
    const tWord = count === 1 ? "Trovato" : "Trovati";
    return `${tWord} ${count} ${sWord} di WoWCensus in ${versions} ${vWord} di gioco.`;
  }
  const sWord = count === 1 ? "save" : "saves";
  const vWord = versions === 1 ? "game version" : "game versions";
  return `Found ${count} WoWCensus ${sWord} in ${versions} ${vWord}.`;
}

export function getEditionName(editionKey) {
  const langEditions = translations[activeLanguage]?.editions || translations.en.editions;
  return langEditions[editionKey] || translations.en.editions[editionKey] || String(editionKey || "").replace(/_/g, " ");
}

export function applyTranslations() {
  document.querySelectorAll("[data-i18n]").forEach((el) => {
    const key = el.dataset.i18n;
    if (key) {
      el.textContent = t(key);
    }
  });
  document.querySelectorAll("[data-i18n-placeholder]").forEach((el) => {
    const key = el.dataset.i18nPlaceholder;
    if (key) {
      el.placeholder = t(key);
    }
  });
  document.querySelectorAll("[data-i18n-title]").forEach((el) => {
    const key = el.dataset.i18nTitle;
    if (key) {
      el.title = t(key);
    }
  });
}
