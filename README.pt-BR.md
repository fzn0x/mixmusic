<p align="center"><a href="README.md">English</a> · <a href="README.hi.md">हिन्दी</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.id.md">Bahasa Indonesia</a> · <a href="README.ur.md">اردو</a> · <strong>Português (Brasil)</strong> · <a href="README.bn.md">বাংলা</a> · <a href="README.ru.md">Русский</a> · <a href="README.am.md">አማርኛ</a></p>

<p align="center">
  <img src="brand/icon.png" width="112" height="112" alt="Ícone do MixMusic: dois anéis rosa e um botão de reproduzir sobre fundo preto" />
</p>

<h1 align="center">MixMusic</h1>

<p align="center"><strong>Um único player para YouTube Music e Spotify.</strong></p>

<p align="center">Cliente de YouTube Music e Spotify para macOS, Windows e Linux. O YouTube Music funciona sem conta. Uma busca, uma fila, playlists mistas, letras sincronizadas, temas e plugins.</p>

<p align="center"><sub>Esta é uma tradução do <a href="README.md">README em inglês</a>. Se houver diferenças, vale a versão atual em inglês.</sub></p>

<p align="center">
  <a href="https://github.com/fzn0x/mixmusic/actions/workflows/ci.yml"><img src="https://github.com/fzn0x/mixmusic/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://github.com/fzn0x/mixmusic/actions/workflows/pages.yml"><img src="https://github.com/fzn0x/mixmusic/actions/workflows/pages.yml/badge.svg" alt="Site" /></a>
  <a href="https://github.com/fzn0x/mixmusic/releases/latest"><img src="https://img.shields.io/github/v/release/fzn0x/mixmusic" alt="Última versão" /></a>
  <a href="https://github.com/fzn0x/mixmusic/releases"><img src="https://img.shields.io/github/downloads/fzn0x/mixmusic/total" alt="Downloads" /></a>
  <a href="https://pkg.go.dev/github.com/fzn0x/mixmusic"><img src="https://pkg.go.dev/badge/github.com/fzn0x/mixmusic.svg" alt="Referência do Go" /></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/fzn0x/mixmusic" alt="Versão do Go" /></a>
  <a href="https://awesomemygo.dev/apps/mixmusic"><img src="https://awesomemygo.dev/badge.svg" height="20" alt="Listado no Awesome MyGo" /></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey" alt="macOS, Windows e Linux" />
  <a href="#licença"><img src="https://img.shields.io/badge/license-MIT%20or%20GPL--3.0-blue" alt="MIT ou GPL-3.0" /></a>
</p>

<p align="center">
  <a href="https://fzn0x.github.io/mixmusic/">Site</a> ·
  <a href="https://github.com/fzn0x/mixmusic/releases/latest">Baixar</a> ·
  <a href="https://fzn0x.github.io/mixmusic/docs/">Documentação</a> ·
  <a href="CHANGELOG.md">Changelog</a>
</p>

<p align="center">
  <img src="docs/banner.webp" alt="MixMusic 0.5.9 com a aparência padrão, o layout Blend com o tema Mix, de pé sobre um palco escuro e de frente para você: no meio, tocando agora アイドル, do YOASOBI, contornado em rosa, com romaji embaixo de cada linha da letra e o mini player na frente; uma busca à esquerda e os mixes da biblioteca à direita" />
</p>

## Por que o MixMusic

Se a sua música está dividida entre o YouTube Music e o Spotify, você
costuma precisar de dois apps, com duas buscas, duas filas e duas
bibliotecas. O MixMusic reúne os dois serviços em um só player:

- **Uma busca.** Ele pesquisa no YouTube Music e no Spotify ao mesmo tempo
  e marca cada resultado com o serviço de onde vem.
- **Uma fila e uma biblioteca.** Uma faixa do Spotify pode tocar logo depois
  de uma do YouTube Music, e uma playlist pode ter faixas dos dois.
- **Sem precisar de conta.** O YouTube Music toca assim que você abre o app.
  Entre na sua conta só se quiser a sua biblioteca. O Spotify é opcional.
- **Um programa pequeno.** O MixMusic é um programa em Go que desenha as
  próprias janelas na GPU. Não há navegador dentro dele para baixar nem para
  manter na memória.
- **Do seu jeito.** Vêm quatro layouts e catorze temas, a comunidade
  adiciona mais temas e plugins, e qualquer comando pode ter uma tecla
  sua.
- **Privado.** O MixMusic não tem conta, servidor, análise de uso nem
  telemetria próprios.
- **Livre e de código aberto,** sob a licença MIT ou a GPL 3.0.

## Recursos

### Buscar e tocar

- **Uma busca para os dois serviços.** Resultados do YouTube Music e do
  Spotify lado a lado, cada um marcado com o seu serviço. Os resultados do
  YouTube Music mostram quantas vezes cada faixa foi tocada, para que a
  original se destaque das covers.
- **Uma fila.** Misture faixas dos dois serviços, use aleatório e repetir
  entre elas, e deixe um rádio continuar quando a fila acabar.
- **Explorar.** Lançamentos, músicas em alta, as paradas, humores e
  gêneros, e músicas relacionadas ao que está tocando.
- **Letras sincronizadas** que acompanham a música, ou texto simples, se
  você preferir.
- **Um mini player** que fica por cima das suas outras janelas.

### Sua biblioteca

- **Mixes:** playlists que reúnem faixas de todos os serviços. Importe a
  playlist de um serviço para um mix e pesquise e ordene dentro de qualquer
  um deles.
- **Músicas curtidas** dos dois serviços em uma só lista.
- **Histórico e estatísticas de audição,** dia a dia.
- **Suas contas.** Entre no YouTube Music na própria página do Google para
  ver seu feed inicial, músicas curtidas, playlists, álbuns e artistas, como
  a sua conta ou um de seus canais de marca. Entre no Spotify para ter a
  sua biblioteca de lá. Curtidas, álbuns e playlists salvos e artistas
  seguidos também chegam ao serviço. O que você toca entra no seu
  histórico do YouTube Music, de onde vêm as recomendações dele.

### Spotify, de três jeitos

- **No próprio MixMusic,** na qualidade do Spotify, sem nenhum app do
  Spotify aberto (Premium, no Windows e no macOS).
- **No Spotify Connect:** seu app do Spotify, seu celular ou uma caixa de
  som.
- **Pelo YouTube Music:** o MixMusic toca a gravação da faixa do Spotify no
  YouTube Music, que não exige Premium para tocar.

### Som

- Crossfade, volume uniforme entre as faixas e nivelamento de volume.
- Um equalizador de dez bandas com predefinições.
- Um visualizador.
- Um cache offline das faixas que você tocou.

### No seu desktop

- Teclas de mídia e os controles de mídia do sistema no Windows, macOS e
  Linux.
- Atalhos globais seus para tocar, pausar, pular, mudar o volume, silenciar,
  curtir ou descurtir o que toca ou mostrar o MixMusic, seja qual for a
  janela na frente.
- Os botões do player na miniatura da barra de tarefas no Windows.
- Controles na bandeja do sistema, notificações de novas faixas e início
  junto com o sistema.
- Links `mixmusic://`.
- **Last.fm:** cada faixa que você ouve entra no seu perfil, e os scrobbles
  feitos offline são enviados assim que você volta a ficar online.
- **Discord:** seu perfil mostra o que está tocando, com a capa e o tempo
  restante.
- **Atualizações:** o MixMusic procura uma nova versão ao iniciar e a cada
  poucas horas, verifica a assinatura dela e a instala quando você
  mandar, ou sozinho, se você escolher assim.

### Deixe do seu jeito

- **Layouts.** O Blend, o padrão, tem formas arredondadas e movimento
  elástico em fileiras densas de capas grandes. O Expressive, inspirado no
  Material 3 Expressive, é divertido, com preenchimentos tonais, botões em
  formato de pílula e uma barra do player flutuante. O Crate é contido, para
  que as capas façam o trabalho, e o Tiles é o primeiro visual do MixMusic,
  com blocos planos e palavras gigantes em minúsculas.
- **Temas.** Catorze embutidos, cada um em qualquer layout, com efeitos que
  você pode ajustar, um tema para o modo claro e outro para o modo escuro, e
  uma cor de destaque sua ou tirada da capa que está tocando. Material e
  Material Dark tiram todas as cores da capa.
- **Temas e plugins da comunidade,** a um clique de distância no marketplace.
- **Plugins** adicionam páginas, comandos, ações de faixa e widgets por
  meio de uma API tipada, desenhados no seu tema.
- **Teclado em primeiro lugar.** Atalhos para tudo o que é comum, uma tecla
  sua para qualquer comando, inclusive os dos plugins, teclas que controlam
  a música de qualquer app e uma paleta de comandos (<kbd>Ctrl</kbd>
  <kbd>K</kbd>) para o resto.
- **Um arquivo de configurações.** Suas configurações ficam em
  `settings.json`, na sua pasta de dados (`%AppData%\MixMusic` no Windows,
  `~/Library/Application Support/MixMusic` no macOS, `~/.config/MixMusic` no
  Linux), e você pode guardá-lo junto com seus dotfiles. Salve-o com o
  MixMusic aberto e ele vale na hora; Settings › Desktop › Settings file
  (Configurações › Desktop › Arquivo de configurações) o abre.

## Uma olhada

<p align="center">
  <img src="docs/look-listen.webp" alt="As páginas do MixMusic no layout Blend, de pé sobre um palco escuro e de frente para você: no meio, tocando agora アイドル, do YOASOBI, contornado em rosa, cada linha da letra com romaji e uma tradução em inglês embaixo; o álbum THE BOOK 3 à esquerda e a página de um podcast seguido à direita" />
</p>

<p align="center"><sub><b>Ouça.</b> No meio, tocando agora アイドル, do YOASOBI: cada linha se acende quando é cantada, com romaji e uma tradução embaixo. À esquerda, um álbum; à direita, um podcast que você segue.</sub></p>

<p align="center">
  <img src="docs/look-yours.webp" alt="Mais do MixMusic, de pé sobre um palco escuro e de frente para você: Settings › Appearance (Configurações › Aparência) com os quatro layouts no meio, contornados em rosa; um tema Material colorido pela capa que está tocando à esquerda, e os temas da comunidade no marketplace à direita" />
</p>

<p align="center"><sub><b>Deixe do seu jeito.</b> No meio, os quatro layouts: Blend, o padrão, Expressive, Crate e Tiles, acima dos temas. À esquerda, um tema Material que tira as cores da capa que está tocando. À direita, temas da comunidade no marketplace.</sub></p>

## Instalação

Baixe o arquivo para o seu sistema na
[última versão](https://github.com/fzn0x/mixmusic/releases/latest):

| Sistema | Download |
|---|---|
| macOS 12 ou posterior (Apple silicon e Intel) | [`mixmusic-<version>-macos-universal.dmg`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Windows 10 e 11 (x64, Arm), instalador | [`mixmusic-<version>-windows-<arch>-setup.exe`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Windows, portátil, sem instalação | [`mixmusic-<version>-windows-<arch>-portable.zip`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Debian, Ubuntu e derivadas | [`mixmusic-<version>-linux-<arch>.deb`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Fedora, openSUSE e derivadas | [`mixmusic-<version>-linux-<arch>.rpm`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Arch Linux e derivadas (em breve) | [`mixmusic-bin`](https://aur.archlinux.org/packages/mixmusic-bin) no AUR: `yay -S mixmusic-bin` |
| Qualquer Linux, um único arquivo | [`mixmusic-<version>-linux-<arch>.AppImage`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Qualquer Linux, em sandbox (Flatpak) | [`mixmusic-<version>-linux-<arch>.flatpak`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Qualquer Linux, na sua pasta pessoal | [`mixmusic-<version>-linux-<arch>.tar.gz`](https://github.com/fzn0x/mixmusic/releases/latest), com `install.sh` |

`<arch>` é `amd64` para x64 e `arm64` para Arm. Cada versão lista os
`SHA256SUMS` para você conferir os downloads. Você não precisa dos arquivos
chamados `update-…`, `….delta`, `mixmusic-<version>-macos-…` nem
`-windows-<arch>.tar.gz`: os apps instalados se atualizam a partir deles.

Os instaladores, o zip portátil e o arquivo do Linux se atualizam sozinhos.
O `.deb`, o `.rpm`, o AppImage e o Flatpak não conseguem substituir os
próprios arquivos, então o MixMusic avisa quando há uma versão nova e a
baixa para você. O `mixmusic-bin` do AUR se atualiza junto com o resto do
seu sistema. No zip portátil, descompacte-o em qualquer lugar e abra o
`MixMusic.exe`. No AppImage, torne-o executável (`chmod +x`) e abra-o. No
Flatpak, abra-o com a sua central de programas ou rode
`flatpak install ./mixmusic-<version>-linux-<arch>.flatpak`; na primeira
vez, o Flatpak baixa o runtime do GNOME do Flathub.

Em breve, quando o tap estiver no ar: no Mac, o [Homebrew](https://brew.sh)
instala o MixMusic, e `brew upgrade` o atualiza.

```sh
brew install --cask fzn0x/tap/mixmusic
```

### A primeira execução

- **macOS:** o app ainda não é notarizado pela Apple, então, na primeira vez
  que você o abrir, o macOS diz que não consegue verificá-lo. Abra-o mesmo
  assim em System Settings › Privacy & Security › Open Anyway (Ajustes do
  Sistema › Privacidade e Segurança › Abrir Mesmo Assim), ou rode isto uma
  vez no Terminal:

  ```sh
  xattr -dr com.apple.quarantine /Applications/MixMusic.app
  ```
- **Windows:** o instalador ainda não é assinado, então o SmartScreen pode
  avisar sobre ele: escolha Mais informações e depois Executar assim mesmo.
  Entrar no YouTube Music e tocar Spotify no MixMusic exigem o WebView2
  Runtime, que o Windows 11 já inclui.
- **Linux:** o MixMusic precisa do GTK 3 e do PulseAudio (o PipeWire também
  o atende) ou do ALSA. Entrar no YouTube Music exige o WebKitGTK 4.1, o
  ícone da bandeja exige `libayatana-appindicator3`, e guardar suas contas
  no chaveiro do sistema exige `libsecret`. O `.deb` e o `.rpm` instalam
  tudo isso, e o Flatpak já o traz; para o AppImage e o arquivo compactado:

  ```sh
  # Debian and Ubuntu
  sudo apt install libgtk-3-0 libpulse0 libwebkit2gtk-4.1-0 libayatana-appindicator3-1 libsecret-1-0
  # Fedora
  sudo dnf install gtk3 pulseaudio-libs webkit2gtk4.1 libayatana-appindicator-gtk3 libsecret
  ```
- **Todos os sistemas:** o MixMusic encontra sozinho o áudio da maioria das
  faixas do YouTube Music. Para as demais, como as feitas para crianças, ele
  usa o [yt-dlp](https://github.com/yt-dlp/yt-dlp), que instala para você
  a partir de Settings › Playback (Configurações › Reprodução) e mantém
  atualizado. O yt-dlp precisa de um runtime de JavaScript:
  [Deno](https://deno.com), Node.js, Bun ou QuickJS. O Flatpak já tem o
  Node.js.

### O Flatpak

O Flatpak roda o MixMusic em um sandbox, que enxerga a rede, o servidor de
som, a tela e a GPU, e nada da sua pasta pessoal. O MixMusic guarda suas
configurações e sua biblioteca em `~/.var/app/io.github.fzn0x.mixmusic`,
separado de uma cópia em `.deb`, `.rpm`, AppImage ou arquivo compactado, e
as duas podem rodar ao mesmo tempo. Para trazer as configurações e a
biblioteca de uma cópia, feche as duas, copie `settings.json`,
`library.json`, `queue.json` e a pasta `addons` de `~/.config/MixMusic` para
`~/.var/app/io.github.fzn0x.mixmusic/config/MixMusic` e entre nas suas
contas de novo: as contas ficam no chaveiro da outra cópia.

No Flatpak, o início junto com o sistema passa pelo seu desktop, que pode
perguntar antes, e um yt-dlp seu precisa estar em
`~/.var/app/io.github.fzn0x.mixmusic`, pois o sandbox não enxerga nenhuma
outra pasta. Um app do Discord que não seja Flatpak só mostra o que está
tocando se já estava rodando quando o MixMusic começou, até ser reiniciado;
o Discord como Flatpak funciona nos dois casos. O Flatpak é novo: conte em
uma [issue](https://github.com/fzn0x/mixmusic/issues) como ele roda no seu
desktop.

### Contas (todas opcionais)

- **YouTube Music:** escolha Sign in to YouTube Music (Entrar no YouTube
  Music) em Settings › Accounts (Configurações › Contas) e entre no Google
  na janela que abrir. O que você digita vai só para o Google. O MixMusic
  guarda a sessão criptografada, com uma chave no chaveiro do seu sistema,
  quando há um. Onde essa janela não puder abrir, ou o Google a recusar,
  entre com o seu próprio navegador, que o MixMusic abre com um perfil novo
  só dele e apaga quando você entra; importe a sessão de um navegador em que
  você entrou (o MixMusic a lê com o yt-dlp e guarda só os cookies do
  YouTube); ou cole o cabeçalho Cookie de uma aba music.youtube.com em que
  você já entrou.
- **Spotify:** o Spotify deixa um app atender até cinco usuários, e só
  enquanto o dono dele tem Premium, então você entra com um app do Spotify
  seu. Leva dois minutos: [docs/spotify.md](docs/spotify.md) guia você
  pelo processo.
- **Last.fm:** crie uma conta de API em
  [last.fm/api/account/create](https://www.last.fm/api/account/create)
  (qualquer nome, sem URL de callback), cole a chave de API e o segredo
  compartilhado em Settings › Accounts e aprove o MixMusic no seu navegador.
- **Discord:** enquanto o app do Discord roda, seu perfil mostra o que você
  ouve. Desative em Settings › Desktop (Configurações › Desktop).

Algo não funciona? [docs/troubleshooting.md](docs/troubleshooting.md)
cobre os problemas comuns e os problemas conhecidos.

## Compilar a partir do código-fonte

Você precisa do [Go](https://go.dev/dl/) 1.27.3 ou posterior, do
[Bun](https://bun.sh) 1.2 ou posterior, que roda a linha de comando do MyGo,
e do Git. O MyGo não precisa de compilador C, então qualquer sistema compila
o MixMusic para todos os sistemas.

### Windows

```powershell
winget install GoLang.Go
winget install Oven-sh.Bun
winget install Git.Git
```

Abra um novo terminal e depois:

```powershell
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

Entrar nas contas e tocar Spotify exigem o WebView2 Runtime, que o Windows 11
já inclui. No Windows 10, instale-o pela
[Microsoft](https://developer.microsoft.com/microsoft-edge/webview2/).

### macOS

Com o [Homebrew](https://brew.sh):

```sh
brew install go oven-sh/bun/bun
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

### Linux

Instale o Go a partir de [go.dev/dl](https://go.dev/dl/) (os pacotes das
distribuições costumam ser mais antigos que o 1.27.3), depois o Bun e as
bibliotecas com que o MixMusic roda:

```sh
curl -fsSL https://bun.sh/install | bash
# Debian and Ubuntu
sudo apt install git libgtk-3-0 libpulse0 libwebkit2gtk-4.1-0 libayatana-appindicator3-1 libsecret-1-0
# Fedora
sudo dnf install git gtk3 pulseaudio-libs webkit2gtk4.1 libayatana-appindicator-gtk3 libsecret
```

Depois:

```sh
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

### Nix

Com o [Nix](https://nixos.org), `nix develop` dá a você as ferramentas com
que as versões são compiladas, nas versões que o `flake.lock` fixa: Go, Bun,
NSIS, GoReleaser e Zola. `nix build` compila o MixMusic para Linux a partir
do código-fonte, em `result/`.

### Desenvolvimento

`bun run dev` compila o app, o inicia e o recompila conforme você edita. Os
dados dele ficam separados dos de um MixMusic instalado, sob o nome
`MixMusic Dev`.

```sh
bun run build:plugins          # the plugins' JavaScript, from their TypeScript
go test ./...                  # the app's tests
bun run typecheck              # the plugin SDK and the community's plugins
go run ./tools/registry -check # the community registry
bun run build                  # the packaged app for this system, in build/
bun run build -- -platform windows/amd64,linux/amd64,linux/arm64
```

No shell do Nix, compile os downloads do Linux com
`.github/scripts/build.sh linux/amd64,linux/arm64`, como as versões fazem.
Um simples `bun run build` ali gera um programa Linux que pede o loader do
próprio Nix, e ele só inicia no NixOS. `go run ./tools/elfloader build/linux-amd64`
confere uma compilação.

O [site](https://fzn0x.github.io/mixmusic/) fica em `site/`, compilado com o
[Zola](https://www.getzola.org), e os guias em `docs/` são as páginas
`/docs/` dele. `go run ./tools/sitedocs && zola --root site serve` mostra
uma prévia. As versões são compiladas no shell do Nix e publicadas pelo
[GoReleaser](https://goreleaser.com) (`.goreleaser.yaml`).

[docs/architecture](docs/architecture/README.md) explica como as partes se
encaixam.

### Estrutura

O app é o pacote raiz, com os pacotes de `internal/` abaixo dele. Os pacotes
fora de `internal/` podem ser importados por qualquer pessoa e estão
documentados no [pkg.go.dev](https://pkg.go.dev/github.com/fzn0x/mixmusic).

```
*.go                 the app and its windows: the pages, the player bar, the
                     mini player, the tray, media keys, deep links
fonts/               the typefaces the windows draw with
internal/app         the services the windows call, and the desktop integration
internal/player      the queue, played by the audio engine or on Spotify's devices
internal/audio       MixMusic's audio engine, and the system's audio output
internal/plugins     plugins, each in a JavaScript engine of its own
internal/youtube     YouTube Music's catalog, and your account there
internal/spotify     the Spotify Web API, its sign-in and Spotify Connect
internal/stream      audio streams, from YouTube or yt-dlp, opened for the engine
internal/match       Spotify tracks matched to their YouTube Music recordings
internal/romaji      Japanese lyrics in romaji, kanji read by kagome
internal/library     liked and disliked songs, mixes, history and stats
internal/lastfm      scrobbling to Last.fm
internal/discord     what plays, on your Discord profile
internal/diag        the log, without secrets, and the reports of crashes
internal/addons      themes and plugins, and the community registry
builtin/themes       the themes MixMusic ships with
public/              the page of Spotify's player, on Windows
sdk/                 the types of the plugin API
community/           community themes and plugins, and their registry
```

Os pacotes que você pode importar:

```
opus/                an Opus decoder in pure Go, a port of libopus's
aac/                 an AAC-LC decoder in pure Go
ac3/                 an AC-3 and E-AC-3 decoder in pure Go, written from ATSC A/52
webm/                the audio of WebM files: packets, times and seeking
mp4/                 the audio of MP4 files: packets, times and seeking
resample/            sample rate conversion, with Kaiser-windowed sincs
lyrics/              synced lyrics from Unison and LRCLIB, and translations
brand/               MixMusic's logo, for code that draws it
```

## Comunidade

Qualquer pessoa pode instalar temas e plugins da comunidade pelo marketplace,
o ícone de loja na barra lateral, e você pode adicionar os seus.

### Crie um tema

Um tema é um arquivo `theme.json` que define cores, fontes e cantos, que
aparecem no layout que o usuário escolheu.
[docs/themes.md](docs/themes.md) lista todos os tokens e mostra um tema
completo. Para testar o seu, coloque a pasta dele na pasta de temas
(Settings › Appearance › open themes folder, ou Configurações › Aparência ›
abrir pasta de temas) e escolha-o em Settings › Appearance.

### Crie um plugin

Um plugin é um módulo pequeno, escrito em TypeScript ou JavaScript, que
adiciona páginas, comandos, ações de faixa e widgets.
[docs/plugins.md](docs/plugins.md) documenta a API, com seus
[tipos](sdk/index.d.ts) para o seu editor. O
[sleep timer](community/plugins/sleep-timer) e o
[spectrum](community/plugins/spectrum) da comunidade são exemplos
funcionais.

### Compartilhe

1. Faça um fork deste repositório.
2. Adicione uma pasta: `community/themes/<id>/` para um tema, ou
   `community/plugins/<id>/` para um plugin, com o `plugin.json` e o
   `index.ts` dele.
3. Rode `bun run build:plugins`, `go run ./tools/registry` e
   `go run ./tools/registry -check`.
4. Abra um pull request.

Depois que ele for integrado, o CI o publica na
[branch registry](https://github.com/fzn0x/mixmusic/tree/registry), e ele
aparece no marketplace de todo mundo.
[community/README.md](community/README.md) traz os detalhes.

## Privacidade

O MixMusic não tem conta, servidor, análise de uso nem telemetria próprios.
Ele se comunica com:

- **YouTube e Google:** o catálogo do YouTube Music, seus streams e o quão
  alto está cada faixa, pedidos sem login, e as capas; alguns streams pelo
  yt-dlp. Com login, a sua biblioteca lá e, a menos que você desative em
  Settings › Accounts (Configurações › Contas), as faixas que você toca, que
  entram no seu histórico do YouTube Music.
- **Spotify,** depois que você entra: sua biblioteca, seus dispositivos do
  Spotify Connect e o player do Spotify, carregado de `sdk.scdn.co`, quando
  o Spotify toca no MixMusic.
- **Unison:** o ID de vídeo da faixa que está tocando ou, sem ele, o título,
  os artistas, o álbum e a duração, para encontrar a letra. Somente
  enquanto você deixar ligado traduzir, ou romanizar, ao lado da letra, a
  letra da faixa que está tocando, com o ID de vídeo dela, que o Unison
  manda o Google traduzir.
- **LRCLIB:** o título, os artistas, o álbum e a duração da faixa que está
  tocando, para encontrar a letra.
- **GitHub:** uma verificação de nova versão ao iniciar e a cada poucas
  horas, que você pode desativar em Settings › About (Configurações ›
  Sobre); o registro da comunidade quando você abre o marketplace; os
  downloads do yt-dlp; e, na primeira vez que você liga o romaji ao lado
  de letras em japonês, o dicionário das leituras dos kanji (11,7 MB). O
  romaji é feito no seu computador: as letras não são enviadas.
- **Last.fm,** depois que você o conecta: o que você toca.
- **Discord,** a menos que você desative: o que está tocando, enviado ao app
  do Discord no seu computador, que o mostra no seu perfil.

O que ele guarda no seu computador:

- Suas configurações e sua biblioteca (músicas curtidas e descurtidas,
  mixes, histórico e buscas recentes), na pasta que Settings › About
  indica.
- As faixas que você tocou, no cache, na quantidade que você permitir em
  Settings › Playback, e o quão alto o YouTube as mediu.
- As capas que você viu, até 256 MB.
- As traduções de letras que você ligou, por faixa e idioma, e o dicionário
  com que o romaji lê os kanji, depois que você o ligou.
- Suas contas, inclusive a sessão do YouTube Music, no chaveiro do sistema.
  No Windows, quando o Gerenciador de Credenciais não consegue guardar uma,
  ela vai para um arquivo criptografado para a sua conta do Windows; no
  Linux sem chaveiro, em arquivos que só o seu usuário pode ler.
- O log, sem as suas contas.

A página de histórico da biblioteca apaga o que você tocou, Settings ›
Playback apaga o cache e as capas, e sair de um serviço apaga o login dele.

No Windows, o visualizador ouve o áudio do Spotify tocando neste
computador, no MixMusic ou no app do Spotify, por meio do process loopback
do Windows, que ouve só esse app. No macOS 14.2 ou posterior, ele ouve o
Spotify tocando no MixMusic por meio de um process tap, depois que você
permite, como o macOS pede. Ele analisa o áudio na memória e nunca o grava
nem o envia.

## Créditos

O MixMusic é construído sobre estes projetos:

- **[MyGo](https://github.com/egoist/mygo)**, o framework de desktop com que
  o MixMusic é feito. O MixMusic usa a UI nativa do MyGo: o MyGo desenha cada
  janela por conta própria, em Go, sem webview, das páginas e da barra do
  player ao mini player. A linha de comando dele empacota o app, com os
  instaladores e as atualizações assinadas pelas quais ele se atualiza.
- **[Go](https://go.dev)**, a linguagem do app, com
  [purego](https://github.com/ebitengine/purego), que chama as bibliotecas
  do sistema sem cgo, e [golang.org/x/image](https://pkg.go.dev/golang.org/x/image)
  e [golang.org/x/text](https://pkg.go.dev/golang.org/x/text).
- **[Sobek](https://github.com/grafana/sobek)**, o motor de JavaScript em que
  cada plugin roda.
- **[Nix](https://nixos.org)**, que fixa as ferramentas com que as versões
  são compiladas e compila o MixMusic para Linux a partir do código-fonte.
- **[GoReleaser](https://goreleaser.com)**, que publica as versões, e o
  [nFPM](https://nfpm.goreleaser.com) dele, que gera os pacotes RPM.
- **[Bun](https://bun.sh)**, que roda a compilação, e
  **[TypeScript](https://www.typescriptlang.org)**, que compila os plugins e
  verifica os tipos do SDK de plugins.
- **[Zola](https://www.getzola.org)**, que compila o site.
- **[NSIS](https://nsis.sourceforge.io)**, que faz os instaladores do
  Windows, e o [appimagetool](https://github.com/AppImage/appimagetool), os
  AppImages.
- **[yt-dlp](https://github.com/yt-dlp/yt-dlp)**, que encontra o áudio das
  faixas que o MixMusic não consegue encontrar sozinho.
- **[Unison](https://unison.betterlyrics.org)**, o banco de dados aberto e
  colaborativo de letras, sob a ODbL, e **[LRCLIB](https://lrclib.net)**, o
  banco de dados aberto de letras sincronizadas.
- **[kagome](https://github.com/ikawaha/kagome)**, o analisador morfológico
  de japonês que lê os kanji das letras para gerar o romaji, com o
  dicionário IPA do
  [kagome-dict](https://github.com/ikawaha/kagome-dict), feito a partir do
  mecab-ipadic, que o MixMusic baixa na primeira vez que o romaji é ligado.
- **[libopus](https://opus-codec.org)**, de quem o decodificador Opus é um
  port, e **[SharpJaad](https://github.com/jimm98y/SharpJaad)**, o port em
  C# do [JAAD](https://sourceforge.net/projects/jaadec/) de que o
  decodificador AAC deriva.
- **[FFmpeg](https://ffmpeg.org)**, com o qual os testes dos decodificadores
  comparam.
- **[Selawik](https://github.com/microsoft/Selawik)**,
  **[Quicksand](https://github.com/andrew-paglinawan/QuicksandFamily)**,
  **[Roboto Flex](https://github.com/googlefonts/roboto-flex)** e
  **[Archivo](https://github.com/Omnibus-Type/Archivo)**, as fontes das
  janelas.
- **[Material Color Utilities](https://github.com/material-foundation/material-color-utilities)**,
  de quem a ciência de cor HCT, da qual vêm as cores dos temas Material, é um
  port.
- **[Kevin MacLeod](https://incompetech.com)**, cuja música, sob
  [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), toca no site.

## Contribuindo

Correções, recursos, temas, plugins e relatos de bugs são todos bem-vindos.

- **Relate um problema** em Settings › About › report a problem
  (Configurações › Sobre › relatar um problema). Ele abre uma nova issue no
  GitHub com sua versão, sistema, tema e plugins já preenchidos. Ou
  [abra uma issue](https://github.com/fzn0x/mixmusic/issues/new) você mesmo.
- **Envie uma mudança** como um pull request. O
  [CONTRIBUTING.md](CONTRIBUTING.md) explica as verificações a rodar, o
  estilo dos commits e a entrada no changelog que uma mudança que as pessoas
  vão notar exige.
- **Compartilhe um tema ou um plugin:** veja [Comunidade](#comunidade).

O [CONTRIBUTORS.md](CONTRIBUTORS.md) agradece às pessoas cujo código, ideias
e relatos moldaram o MixMusic.

## Licença

O MixMusic tem licença dupla: use-o sob a [licença MIT](LICENSE-MIT) ou a
[GNU General Public License, versão 3](LICENSE-GPL), a que você preferir.
As contribuições são aceitas sob as duas. As fontes em `fonts` estão sob a
SIL Open Font License: a do [Selawik](fonts/OFL.txt), a do
[Quicksand](fonts/quicksand-OFL.txt), a do
[Roboto Flex](fonts/robotoflex-OFL.txt) e a do
[Archivo](fonts/archivo-OFL.txt). O decodificador Opus é um port do
libopus, sob [a licença BSD dele](opus/LICENSE.libopus), o decodificador AAC
deriva do SharpJaad, sob [a licença MIT dele](aac/LICENSE.sharpjaad), e a
ciência de cor HCT é um port do Material Color Utilities, sob
[a licença Apache dele](internal/hct/LICENSE.material-color-utilities).

O MixMusic não é afiliado, endossado nem patrocinado pelo Spotify ou pelo
Google. Ele toca o que esses serviços oferecem a seus usuários, pelas
interfaces dos próprios serviços; use-o como os termos deles permitirem.
Spotify e YouTube Music são marcas registradas de seus donos.
