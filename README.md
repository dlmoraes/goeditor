# 🚀 GoEdit

Um editor de texto para terminal (TUI) incrivelmente rápido, leve e inspirado no Neovim, escrito 100% em Go. Construído para sobreviver às peculiaridades dos terminais modernos (como o Windows Terminal e Cmder) sem perder a elegância.

## ✨ Funcionalidades (O que já temos)

- **Syntax Highlighting Dinâmico:** Suporte nativo a dezenas de linguagens (Go, Python, JS, Lua, etc.) alimentado pelo motor Chroma.
- **Explorador de Arquivos Integrado:** Uma gaveta lateral (`File Tree`) para navegar pelo seu projeto sem sair do editor.
- **Múltiplos Buffers (Abas):** Abra vários arquivos simultaneamente e alterne entre eles em milissegundos.
- **Paleta de Comandos Estilo Vim:** Pressione `Ctrl + P` para abrir o rodapé executivo e rodar comandos como `:w`, `:q`, `:wq`.
- **Autocompletar Inteligente (Inline):** Comece a digitar uma palavra e pressione `Tab` (ou `Ctrl+Espaço`) para ver sugestões baseadas no conteúdo do próprio arquivo.
- **Histórico Completo:** Desfazer (`Undo`) e Refazer (`Redo`) com suporte a snapshots de memória.
- **Calibrador de Tela (Anti-Windows Bug):** Ajuste dinâmico de margem em tempo real para evitar que terminais com scroll automático quebrem a interface.
- **Busca e Substituição:** Motor rápido de Find (`Ctrl+F`) e Replace (`Ctrl+R`).

## 📦 Como Instalar e Compilar

Como o GoEdit é feito em Go, ele compila para um único arquivo binário (sem dependências pesadas, sem Node.js, sem DLLs).

**1. Clone o repositório:**
```bash
git clone [https://github.com/seu-usuario/goeditor.git](https://github.com/seu-usuario/goeditor.git)
cd goeditor
```

**2. Compile o executável otimizado (Leve e Rápido):**

Para Linux/macOS:
```bash
go build -ldflags="-s -w" -o goedit
```

Para Windows (PowerShell/CMD):
```powershell
go build -ldflags="-s -w" -o goedit.exe
```

**3. Uso:**
```bash
./goedit               # Abre um buffer vazio no diretório atual
./goedit main.go       # Abre um arquivo específico
```

*(Dica: Adicione a pasta do executável à variável `PATH` do seu sistema para rodar `goedit` de qualquer lugar!)*

## ⌨️ Atalhos de Teclado (Keybindings)

O GoEdit foi desenhado para manter as suas mãos no teclado o tempo todo.

### Globais & Layout
| Tecla | Ação |
| :--- | :--- |
| `Ctrl + E` | Ocultar / Mostrar a gaveta do Explorador de Arquivos |
| `Ctrl + B` | Alternar o foco entre a Gaveta e o Editor |
| `Alt + O` | Encolher a tela (Calibrador - Aumenta margem inferior) |
| `Alt + P` | Expandir a tela (Calibrador - Diminui margem inferior) |

### Navegação de Abas (Buffers)
| Tecla | Ação |
| :--- | :--- |
| `Alt + PageDown` ou `Alt + →` | Próxima Aba |
| `Alt + PageUp` ou `Alt + ←` | Aba Anterior |
| `Ctrl + W` | Fechar Aba Atual |

### Edição Básica
| Tecla | Ação |
| :--- | :--- |
| `Ctrl + S` | Guardar (Save) |
| `Ctrl + X` | Sair do Editor (se não houver seleção) ou Cortar |
| `Ctrl + C` / `Ctrl + V` | Copiar / Colar (Área de transferência interna) |
| `Ctrl + Z` / `Ctrl + Y` | Undo / Redo |
| `Ctrl + /` (ou `Ctrl + _`) | Alternar Comentário na linha atual |
| `Ctrl + K` | Excluir a linha atual inteira |
| `Ctrl + D` | Duplicar a linha atual |
| `Alt + ↑` / `Alt + ↓` | Mover a linha atual para cima/baixo |

### Comandos Avançados
| Tecla | Ação |
| :--- | :--- |
| `Ctrl + P` | Abrir Paleta de Comandos (`:w`, `:q`, `:wq`, `:q!`) |
| `Tab` | Autocompletar (se o cursor estiver após uma palavra) ou Inserir espaços |
| `Ctrl + Espaço` (ou `Ctrl + @`) | Forçar abertura do Autocompletar |
| `Ctrl + F` | Buscar Palavra |
| `Ctrl + N` | Próximo resultado da busca |
| `Ctrl + R` | Substituir Palavra (Replace) |
| `Ctrl + G` | Ir para Linha (Goto) |

## 🛣️ Roadmap (Próximos Passos)

- [x] Motor de Renderização TUI (Bubble Tea)
- [x] Syntax Highlighting (Chroma)
- [x] Múltiplos Buffers e Sidebar
- [x] Comandos estilo Neovim
- [x] Autocompletar Dinâmico Inline
- [ ] **Fase 3: Motor Lua (Sprint 14)** - Suporte a scripts `.lua` para customização e criação de plugins (usando `gopher-lua`).

---

**Desenvolvido com 🩵 e Go.**
