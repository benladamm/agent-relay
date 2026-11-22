package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/benladamm/agent-relay/internal/adapter"
	"github.com/benladamm/agent-relay/internal/bundle"
	"github.com/benladamm/agent-relay/internal/model"
	"github.com/benladamm/agent-relay/internal/relay"
)

type Screen int

const (
	ScreenProjects Screen = iota
	ScreenSessions
	ScreenImport
	ScreenReceive
)

// UI Theme Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#6C5CE7")).
			Padding(0, 1)

	subHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A29BFE")).
			Italic(true)

	harnessPillActive = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#00B894")).
				Background(lipgloss.Color("#1A382B")).
				Padding(0, 1).
				MarginRight(1)

	harnessPillInactive = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#636E72")).
				Background(lipgloss.Color("#2D3436")).
				Padding(0, 1).
				MarginRight(1)

	tabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#6C5CE7")).
			Padding(0, 1).
			MarginRight(1)

	tabInactive = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#B2BEC3")).
			Background(lipgloss.Color("#2D3436")).
			Padding(0, 1).
			MarginRight(1)

	selectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#00FFAA")).
				Background(lipgloss.Color("#25283D")).
				PaddingLeft(1)

	normalRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DFE6E9")).
			PaddingLeft(1)

	badgeLimit = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF7675")).
			Background(lipgloss.Color("#4A1C1C")).
			Padding(0, 1)

	badgeDone = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#B2BEC3")).
			Background(lipgloss.Color("#2D3436")).
			Padding(0, 1)

	badgeClaude = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#D63031")).
			Background(lipgloss.Color("#3B1818")).
			Padding(0, 1)

	badgeAntigravity = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#0984E3")).
				Background(lipgloss.Color("#10253E")).
				Padding(0, 1)

	badgeCursor = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00CEC9")).
			Background(lipgloss.Color("#0B3534")).
			Padding(0, 1)

	badgeWindsurf = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#E84393")).
			Background(lipgloss.Color("#3A1326")).
			Padding(0, 1)

	badgeCline = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FDCB6E")).
			Background(lipgloss.Color("#3D3216")).
			Padding(0, 1)

	badgeAider = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#A29BFE")).
			Background(lipgloss.Color("#25203D")).
			Padding(0, 1)

	cardBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#6C5CE7")).
		Padding(1).
		MarginTop(1)

	codeBox = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("#00FFAA")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Padding(1, 2).
		MarginTop(1).
		MarginBottom(1)

	successBanner = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#00B894")).
			Foreground(lipgloss.Color("#00B894")).
			Padding(1).
			MarginTop(1)

	helpFooter = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#636E72")).
			MarginTop(1)
)

type Model struct {
	registry      *adapter.Registry
	harnesses     []model.HarnessInfo
	projects      []model.Project
	sessions      []model.Session
	currentScreen Screen

	// Navigation & selection
	projectCursor int
	sessionCursor int
	selectedProj  *model.Project

	// Filtering & search
	harnessFilterIndex int
	harnessFilters     []string
	searchInput        textinput.Model
	isSearching        bool

	// Import & Relay inputs
	importBundles []string
	importCursor  int
	receiveInput  textinput.Model
	shareCode     string

	// Notifications
	statusMsg string
	errMsg    string
	width     int
	height    int
}

func NewModel(r *adapter.Registry) Model {
	tiSearch := textinput.New()
	tiSearch.Placeholder = "Rechercher un projet par nom ou chemin... (Entrée pour valider)"
	tiSearch.CharLimit = 150
	tiSearch.Width = 50

	tiReceive := textinput.New()
	tiReceive.Placeholder = "Entrez le code de partage (ex: ABC123-XYZ456)..."
	tiReceive.CharLimit = 80
	tiReceive.Width = 50

	harnesses := r.DetectAllHarnesses()
	filters := []string{"all"}
	for _, h := range harnesses {
		if h.Detected {
			filters = append(filters, h.ID)
		}
	}

	allProjects := r.DetectAllProjects("all", "")

	return Model{
		registry:           r,
		harnesses:          harnesses,
		projects:           allProjects,
		currentScreen:      ScreenProjects,
		projectCursor:      0,
		sessionCursor:      0,
		harnessFilterIndex: 0,
		harnessFilters:     filters,
		searchInput:        tiSearch,
		receiveInput:       tiReceive,
		isSearching:        false,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		// 1. Text input in search mode
		if m.isSearching {
			switch msg.String() {
			case "esc":
				m.isSearching = false
				m.searchInput.Blur()
				return m, nil
			case "enter":
				m.isSearching = false
				m.searchInput.Blur()
				m.refreshProjects()
				return m, nil
			default:
				m.searchInput, cmd = m.searchInput.Update(msg)
				m.refreshProjects()
				return m, cmd
			}
		}

		// 2. Text input in receive mode
		if m.currentScreen == ScreenReceive {
			switch msg.String() {
			case "esc":
				m.currentScreen = ScreenProjects
				m.receiveInput.Blur()
				return m, nil
			case "enter":
				code := strings.TrimSpace(m.receiveInput.Value())
				if code == "" {
					m.errMsg = "Veuillez saisir un code valide"
					return m, nil
				}
				m.statusMsg = "Téléchargement et déchiffrement du bundle en cours..."
				plainBytes, err := relay.DownloadEncryptedBundle(code)
				if err != nil {
					m.errMsg = fmt.Sprintf("Erreur téléchargement : %v", err)
					return m, nil
				}

				// Target project path
				targetPath := "."
				if m.selectedProj != nil && m.selectedProj.Path != "" {
					targetPath = m.selectedProj.Path
				}

				// Find matching adapter
				var ad adapter.Adapter
				if peek, err := bundle.PeekManifest(plainBytes); err == nil {
					ad, _ = m.registry.Get(peek.AgentName)
				}
				if ad == nil {
					ad, _ = m.registry.Get("claude-code")
				}
				manifest, err := bundle.ImportBytes(ad, plainBytes, targetPath)
				if err != nil {
					m.errMsg = fmt.Sprintf("Erreur import : %v", err)
				} else {
					m.statusMsg = fmt.Sprintf("✓ Session %s importée avec succès via Relay !", manifest.SessionID[:8])
					m.currentScreen = ScreenProjects
					m.refreshProjects()
				}
				return m, nil
			default:
				m.receiveInput, cmd = m.receiveInput.Update(msg)
				return m, cmd
			}
		}

		// 3. Import modal screen
		if m.currentScreen == ScreenImport {
			switch msg.String() {
			case "esc":
				m.currentScreen = ScreenProjects
				return m, nil
			case "up", "k":
				if m.importCursor > 0 {
					m.importCursor--
				}
			case "down", "j":
				if m.importCursor < len(m.importBundles)-1 {
					m.importCursor++
				}
			case "enter":
				if len(m.importBundles) > 0 && m.importCursor < len(m.importBundles) {
					bundleFile := m.importBundles[m.importCursor]
					targetPath := "."
					if m.selectedProj != nil && m.selectedProj.Path != "" {
						targetPath = m.selectedProj.Path
					}
					data, _ := os.ReadFile(bundleFile)
					var ad adapter.Adapter
					if peek, err := bundle.PeekManifest(data); err == nil {
						ad, _ = m.registry.Get(peek.AgentName)
					}
					if ad == nil {
						ad, _ = m.registry.Get("claude-code")
					}
					manifest, err := bundle.Import(ad, bundleFile, targetPath)
					if err != nil {
						m.errMsg = fmt.Sprintf("Erreur import : %v", err)
					} else {
						m.statusMsg = fmt.Sprintf("✓ Bundle %s importé avec succès !", manifest.SessionID[:8])
						m.currentScreen = ScreenProjects
						m.refreshProjects()
					}
				}
				return m, nil
			}
			return m, nil
		}

		// 4. Main screens navigation
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "/":
			if m.currentScreen == ScreenProjects {
				m.isSearching = true
				m.searchInput.Focus()
				return m, textinput.Blink
			}

		case "tab":
			if m.currentScreen == ScreenProjects && len(m.harnessFilters) > 0 {
				m.harnessFilterIndex = (m.harnessFilterIndex + 1) % len(m.harnessFilters)
				m.refreshProjects()
				return m, nil
			}

		case "i":
			// Open Import screen: list local .tar.gz bundles
			m.importBundles = findLocalBundles()
			m.importCursor = 0
			m.currentScreen = ScreenImport
			m.statusMsg = ""
			m.errMsg = ""
			return m, nil

		case "g":
			// Open Receive screen
			m.receiveInput.Reset()
			m.receiveInput.Focus()
			m.currentScreen = ScreenReceive
			m.statusMsg = ""
			m.errMsg = ""
			return m, textinput.Blink

		case "s":
			// Send / Relay encrypted bundle
			if m.currentScreen == ScreenSessions && len(m.sessions) > 0 && m.sessionCursor < len(m.sessions) && m.selectedProj != nil {
				sel := m.sessions[m.sessionCursor]
				ad, ok := m.registry.Get(sel.AgentName)
				if ok {
					m.statusMsg = "Chiffrement AES-256 et téléversement sur le relais en cours..."
					data, err := bundle.ExportBytes(ad, sel, m.selectedProj.Path)
					if err != nil {
						m.errMsg = fmt.Sprintf("Erreur export: %v", err)
						return m, nil
					}
					passcode := relay.GeneratePasscode()
					code, err := relay.UploadEncryptedBundle(data, passcode)
					if err != nil {
						m.errMsg = fmt.Sprintf("Erreur envoi relais: %v", err)
					} else {
						m.shareCode = code
						m.statusMsg = fmt.Sprintf("✓ Session chiffrée avec succès !\nCode de partage pour votre autre machine : %s", code)
					}
				}
				return m, nil
			}

		case "esc", "backspace":
			if m.currentScreen == ScreenSessions {
				m.currentScreen = ScreenProjects
				m.statusMsg = ""
				m.errMsg = ""
				m.shareCode = ""
				return m, nil
			}

		case "up", "k":
			if m.currentScreen == ScreenProjects {
				if m.projectCursor > 0 {
					m.projectCursor--
				}
			} else {
				if m.sessionCursor > 0 {
					m.sessionCursor--
					m.statusMsg = ""
				}
			}

		case "down", "j":
			if m.currentScreen == ScreenProjects {
				if m.projectCursor < len(m.projects)-1 {
					m.projectCursor++
				}
			} else {
				if m.sessionCursor < len(m.sessions)-1 {
					m.sessionCursor++
					m.statusMsg = ""
				}
			}

		case "enter":
			if m.currentScreen == ScreenProjects {
				if len(m.projects) > 0 && m.projectCursor < len(m.projects) {
					sel := m.projects[m.projectCursor]
					m.selectedProj = &sel
					sessions, err := m.registry.GetSessions(sel)
					if err == nil {
						m.sessions = sessions
						m.sessionCursor = 0
						m.currentScreen = ScreenSessions
						m.statusMsg = ""
						m.errMsg = ""
						m.shareCode = ""
					} else {
						m.errMsg = fmt.Sprintf("Erreur chargement sessions : %v", err)
					}
				}
			} else if m.currentScreen == ScreenSessions {
				if len(m.sessions) > 0 && m.sessionCursor < len(m.sessions) && m.selectedProj != nil {
					selSession := m.sessions[m.sessionCursor]
					ad, ok := m.registry.Get(selSession.AgentName)
					if ok {
						out, err := bundle.Export(ad, selSession, m.selectedProj.Path, "")
						if err != nil {
							m.errMsg = fmt.Sprintf("Erreur export : %v", err)
						} else {
							m.statusMsg = fmt.Sprintf("✓ Bundle exporté avec succès !\nFichier : %s", out)
						}
					}
				}
			}

		case "r":
			m.harnesses = m.registry.DetectAllHarnesses()
			if m.currentScreen == ScreenProjects {
				m.refreshProjects()
			} else if m.selectedProj != nil {
				sessions, err := m.registry.GetSessions(*m.selectedProj)
				if err == nil {
					m.sessions = sessions
					if m.sessionCursor >= len(m.sessions) {
						m.sessionCursor = len(m.sessions) - 1
					}
					if m.sessionCursor < 0 {
						m.sessionCursor = 0
					}
				}
			}
			m.statusMsg = "Actualisé !"
		}
	}

	return m, nil
}

func findLocalBundles() []string {
	var list []string
	matches, _ := filepath.Glob("relay-*.tar.gz")
	list = append(list, matches...)
	matches2, _ := filepath.Glob("*.tar.gz")
	for _, m := range matches2 {
		found := false
		for _, ex := range list {
			if ex == m {
				found = true
				break
			}
		}
		if !found {
			list = append(list, m)
		}
	}
	return list
}

func (m *Model) refreshProjects() {
	currFilter := m.harnessFilters[m.harnessFilterIndex]
	m.projects = m.registry.DetectAllProjects(currFilter, m.searchInput.Value())
	if m.projectCursor >= len(m.projects) {
		m.projectCursor = len(m.projects) - 1
	}
	if m.projectCursor < 0 {
		m.projectCursor = 0
	}
}

func (m Model) View() string {
	var b strings.Builder

	// Header
	b.WriteString(titleStyle.Render("⚡ AGENT-RELAY"))
	b.WriteString("  ")
	b.WriteString(subHeaderStyle.Render("Hub de synchronisation & transfert de sessions IA multi-OS"))
	b.WriteString("\n\n")

	// Harnesses detected status bar
	b.WriteString("Harnais détectés : ")
	for _, h := range m.harnesses {
		if h.Detected {
			b.WriteString(harnessPillActive.Render(fmt.Sprintf("● %s (%d sessions)", h.DisplayName, h.SessionCount)))
		} else {
			b.WriteString(harnessPillInactive.Render(fmt.Sprintf("○ %s (Non détecté)", h.DisplayName)))
		}
	}
	b.WriteString("\n\n")

	// Screen: Receive
	if m.currentScreen == ScreenReceive {
		b.WriteString("← [Échap] Retour   |   📥 RECEVOIR UN BUNDLE VIA CODE RELAY CHIFHRÉ\n\n")
		b.WriteString("Entrez le code de partage généré sur l'autre machine :\n\n")
		b.WriteString(m.receiveInput.View() + "\n\n")
		b.WriteString(helpFooter.Render("Le bundle sera téléchargé, déchiffré via AES-256 et automatiquement installé sur cet OS.\n\n[Entrée] Télécharger & Installer  •  [Échap] Annuler"))
		return b.String()
	}

	// Screen: Import
	if m.currentScreen == ScreenImport {
		b.WriteString("← [Échap] Retour   |   📦 IMPORTER UN BUNDLE LOCAL (.tar.gz)\n\n")
		if len(m.importBundles) == 0 {
			b.WriteString("  Aucun fichier relay-*.tar.gz trouvé dans le dossier courant.\n")
			b.WriteString("  Déposez une archive de session dans ce dossier puis réessayez.\n\n")
		} else {
			b.WriteString(fmt.Sprintf("Archives détectées (%d) :\n\n", len(m.importBundles)))
			for i, file := range m.importBundles {
				fi, _ := os.Stat(file)
				sizeMB := float64(0)
				if fi != nil {
					sizeMB = float64(fi.Size()) / 1024 / 1024
				}
				line := fmt.Sprintf(" %-45s  (%.2f MB)", file, sizeMB)
				if i == m.importCursor {
					b.WriteString(selectedRowStyle.Render("▶ " + line))
				} else {
					b.WriteString(normalRowStyle.Render("  " + line))
				}
				b.WriteString("\n")
			}
		}
		b.WriteString(helpFooter.Render("\n[↑/↓] Choisir  •  [Entrée] Importer et appliquer  •  [Échap] Annuler\n"))
		return b.String()
	}

	// Screen: Projects Browser
	if m.currentScreen == ScreenProjects {
		b.WriteString("Filtres harnais [Tab] : ")
		for i, f := range m.harnessFilters {
			label := strings.ToUpper(f)
			if f == "all" {
				label = "TOUS"
			}
			if i == m.harnessFilterIndex {
				b.WriteString(tabActive.Render(label))
			} else {
				b.WriteString(tabInactive.Render(label))
			}
		}
		b.WriteString("\n\n")

		// Search input
		if m.isSearching {
			b.WriteString("Recherche [Active] : " + m.searchInput.View() + "\n\n")
		} else {
			searchVal := m.searchInput.Value()
			if searchVal != "" {
				b.WriteString(fmt.Sprintf("Recherche [/] : \"%s\" (Appuyez sur [/] pour modifier)\n\n", searchVal))
			} else {
				b.WriteString(helpFooter.Render("Appuyez sur [/] pour rechercher par mot-clé ou chemin...") + "\n\n")
			}
		}

		if len(m.projects) == 0 {
			b.WriteString("  Aucun projet trouvé avec les filtres actuels.\n")
		} else {
			b.WriteString(fmt.Sprintf("Projets disponibles (%d) :\n\n", len(m.projects)))
			for i, p := range m.projects {
				isSelected := i == m.projectCursor
				hBadge := badgeDone.Render(p.HarnessName)
				switch p.HarnessID {
				case "claude-code":
					hBadge = badgeClaude.Render("Claude Code")
				case "antigravity":
					hBadge = badgeAntigravity.Render("Antigravity")
				case "cursor":
					hBadge = badgeCursor.Render("Cursor")
				case "windsurf":
					hBadge = badgeWindsurf.Render("Windsurf")
				case "cline":
					hBadge = badgeCline.Render("Cline")
				case "aider":
					hBadge = badgeAider.Render("Aider")
				}

				dateStr := p.LastModified.Format("02/01 15:04")
				line := fmt.Sprintf(" %-22s  %s  (%d sessions, %s)  %s", p.DisplayName, hBadge, p.SessionCount, dateStr, p.Path)

				if isSelected {
					b.WriteString(selectedRowStyle.Render("▶ " + line))
				} else {
					b.WriteString(normalRowStyle.Render("  " + line))
				}
				b.WriteString("\n")
			}
		}

		if m.statusMsg != "" {
			b.WriteString(successBanner.Render(m.statusMsg))
			b.WriteString("\n")
		}
		if m.errMsg != "" {
			b.WriteString(badgeLimit.Render(m.errMsg))
			b.WriteString("\n")
		}

		b.WriteString(helpFooter.Render("\n  [↑/↓] Naviguer  •  [Entrée] Ouvrir  •  [i] Importer bundle  •  [g] Recevoir code  •  [Tab] Filtre  •  [/] Chercher  •  [q] Quitter\n"))

	} else if m.currentScreen == ScreenSessions {
		// Screen: Sessions Browser
		projName := m.selectedProj.DisplayName
		hName := m.selectedProj.HarnessName
		b.WriteString(fmt.Sprintf("← [Échap] Retour   |   Projet : %s (%s)\n\n", projName, hName))

		if len(m.sessions) == 0 {
			b.WriteString("  Aucune session trouvée pour ce projet.\n")
		} else {
			b.WriteString(fmt.Sprintf("Conversations enregistrées (%d) :\n\n", len(m.sessions)))
			for i, s := range m.sessions {
				isSelected := i == m.sessionCursor
				shortID := s.ID
				if len(shortID) > 8 {
					shortID = shortID[:8]
				}

				var badge string
				if s.Status == model.StatusRateLimit {
					if !s.ResetsAt.IsZero() {
						badge = badgeLimit.Render(fmt.Sprintf("⛔ QUOTA HIT (Reset %s)", s.ResetsAt.Format("15:04")))
					} else {
						badge = badgeLimit.Render("⛔ QUOTA ATTEINT")
					}
				} else {
					badge = badgeDone.Render("✓ TERMINÉE")
				}

				timeStr := s.LastModified.Format("02/01 15:04")
				sizeMB := float64(s.TotalBytes) / 1024 / 1024
				line := fmt.Sprintf(" %s  [%s]  %s  (%.1f MB, %d msgs) ", shortID, timeStr, badge, sizeMB, s.MessageCount)

				if isSelected {
					b.WriteString(selectedRowStyle.Render("▶ " + line))
				} else {
					b.WriteString(normalRowStyle.Render("  " + line))
				}
				b.WriteString("\n")
			}

			// Details Box
			if m.sessionCursor < len(m.sessions) {
				sel := m.sessions[m.sessionCursor]
				var d strings.Builder
				d.WriteString(fmt.Sprintf("ID Complet : %s\n", sel.ID))
				d.WriteString(fmt.Sprintf("Outils     : %d appels exécutés\n", sel.ToolCount))
				if sel.Summary != "" {
					d.WriteString(fmt.Sprintf("Dernier msg: \"%s\"\n", sel.Summary))
				}
				if sel.Status == model.StatusRateLimit && !sel.ResetsAt.IsZero() {
					timeLeft := time.Until(sel.ResetsAt).Round(time.Minute)
					if timeLeft > 0 {
						d.WriteString(fmt.Sprintf("Réinitialisation quota : dans %s (%s)\n", timeLeft, sel.ResetsAt.Format("15:04 MST")))
					}
				}
				b.WriteString(cardBox.Render(d.String()))
				b.WriteString("\n")
			}
		}

		if m.shareCode != "" {
			b.WriteString(codeBox.Render(fmt.Sprintf("CODE DE PARTAGE RELAY (E2EE) :\n\n  %s  \n\nSur votre autre machine : appuyez sur [g] et entrez ce code !", m.shareCode)))
			b.WriteString("\n")
		}

		if m.statusMsg != "" {
			b.WriteString(successBanner.Render(m.statusMsg))
			b.WriteString("\n")
		}
		if m.errMsg != "" {
			b.WriteString(badgeLimit.Render(m.errMsg))
			b.WriteString("\n")
		}

		b.WriteString(helpFooter.Render("\n  [↑/↓] Naviguer  •  [Entrée] Exporter local (.tar.gz)  •  [s] Envoyer Relay P2P/Cloud  •  [Échap] Retour  •  [q] Quitter\n"))
	}

	return b.String()
}
