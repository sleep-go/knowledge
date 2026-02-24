; Knowledge Base Installer Script
; Requires NSIS 3.0+

!define APPNAME "Offline Knowledge Base"
!define COMPANYNAME "My Company"
!define DESCRIPTION "Offline Local Knowledge Base with Llama.cpp"
; !define VERSIONMAJOR 1
; !define VERSIONMINOR 0
; !define VERSIONBUILD 0

Name "${APPNAME}"
OutFile "KnowledgeBase_Installer.exe"
InstallDir "$LOCALAPPDATA\KnowledgeBase" ; Default to user directory (no admin rights needed)
RequestExecutionLevel user ; No admin rights needed

; Pages
Page directory
Page instfiles

Section "Install"
    SetOutPath $INSTDIR
    
    ; Files to install
    File "knowledge.exe"
    ; Add other files like models if needed
    ; File "models\*.gguf"

    ; Create Shortcuts
    CreateShortCut "$DESKTOP\${APPNAME}.lnk" "$INSTDIR\knowledge.exe"
    
    ; Create Uninstaller
    WriteUninstaller "$INSTDIR\uninstall.exe"
    
    ; Registry keys for Add/Remove Programs (Optional, usually needs admin for HKLM, but HKCU is fine)
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}" "DisplayName" "${APPNAME}"
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}" "UninstallString" "$\"$INSTDIR\uninstall.exe$\""
SectionEnd

Section "Uninstall"
    Delete "$INSTDIR\knowledge.exe"
    Delete "$INSTDIR\uninstall.exe"
    Delete "$DESKTOP\${APPNAME}.lnk"
    RMDir "$INSTDIR"
    DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}"
SectionEnd
