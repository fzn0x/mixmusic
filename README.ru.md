<p align="center"><a href="README.md">English</a> · <a href="README.hi.md">हिन्दी</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.id.md">Bahasa Indonesia</a> · <a href="README.ur.md">اردو</a> · <a href="README.ha.md">Hausa</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.bn.md">বাংলা</a> · <strong>Русский</strong> · <a href="README.am.md">አማርኛ</a></p>

<p align="center">
  <img src="brand/icon.png" width="112" height="112" alt="Значок MixMusic: два розовых кольца и кнопка воспроизведения на чёрном фоне" />
</p>

<h1 align="center">MixMusic</h1>

<p align="center"><strong>Один плеер для YouTube Music и Spotify.</strong></p>

<p align="center">Клиент YouTube Music и Spotify для macOS, Windows и Linux. YouTube Music работает без учётной записи. Один поиск, одна очередь, смешанные плейлисты, синхронизированные тексты песен, темы и плагины.</p>

<p align="center"><sub>Это перевод <a href="README.md">английского README</a>. Если есть расхождения, верна актуальная английская версия.</sub></p>

<p align="center">
  <a href="https://github.com/fzn0x/mixmusic/actions/workflows/ci.yml"><img src="https://github.com/fzn0x/mixmusic/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://github.com/fzn0x/mixmusic/actions/workflows/pages.yml"><img src="https://github.com/fzn0x/mixmusic/actions/workflows/pages.yml/badge.svg" alt="Сайт" /></a>
  <a href="https://github.com/fzn0x/mixmusic/releases/latest"><img src="https://img.shields.io/github/v/release/fzn0x/mixmusic" alt="Последний релиз" /></a>
  <a href="https://github.com/fzn0x/mixmusic/releases"><img src="https://img.shields.io/github/downloads/fzn0x/mixmusic/total" alt="Загрузки" /></a>
  <a href="https://pkg.go.dev/github.com/fzn0x/mixmusic"><img src="https://pkg.go.dev/badge/github.com/fzn0x/mixmusic.svg" alt="Справочник Go" /></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/fzn0x/mixmusic" alt="Версия Go" /></a>
  <a href="https://awesomemygo.dev/apps/mixmusic"><img src="https://awesomemygo.dev/badge.svg" height="20" alt="Есть в списке Awesome MyGo" /></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey" alt="macOS, Windows и Linux" />
  <a href="#лицензия"><img src="https://img.shields.io/badge/license-MIT%20or%20GPL--3.0-blue" alt="MIT или GPL-3.0" /></a>
</p>

<p align="center">
  <a href="https://fzn0x.github.io/mixmusic/">Сайт</a> ·
  <a href="https://github.com/fzn0x/mixmusic/releases/latest">Скачать</a> ·
  <a href="https://fzn0x.github.io/mixmusic/docs/">Документация</a> ·
  <a href="CHANGELOG.md">Список изменений</a>
</p>

<p align="center">
  <img src="docs/banner.webp" alt="MixMusic 0.5.9 в оформлении по умолчанию, макет Blend с темой Mix, стоит прямо на тёмной сцене лицом к вам: в центре играет アイドル группы YOASOBI, обведённая розовым, с ромадзи под каждой строкой текста и мини-плеером перед ней; слева поиск, справа миксы из медиатеки" />
</p>

## Зачем нужен MixMusic

Если ваша музыка разбросана между YouTube Music и Spotify, обычно нужны
два приложения: два поиска, две очереди и две медиатеки. MixMusic
объединяет оба сервиса в одном плеере:

- **Один поиск.** Он ищет сразу в YouTube Music и Spotify и помечает
  каждый результат сервисом, откуда тот родом.
- **Одна очередь и одна медиатека.** Трек из Spotify может играть сразу
  после трека из YouTube Music, а в плейлисте могут лежать треки из обоих.
- **Учётная запись не нужна.** YouTube Music играет, как только вы открыли
  приложение. Входите, только если хотите свою медиатеку. Spotify по
  желанию.
- **Небольшая программа.** MixMusic это одна программа на Go, которая сама
  рисует свои окна на GPU. Внутри нет браузера, который пришлось бы
  скачивать и держать в памяти.
- **Настраивайте под себя.** В комплекте четыре макета и четырнадцать тем,
  сообщество добавляет новые темы и плагины, а у любой команды может быть
  своя клавиша.
- **Приватность.** У MixMusic нет своих учётных записей, сервера,
  аналитики и телеметрии.
- **Свободное ПО с открытым кодом** под лицензией MIT или GPL 3.0.

## Возможности

### Поиск и воспроизведение

- **Один поиск по обоим сервисам.** Результаты YouTube Music и Spotify
  рядом, каждый с пометкой сервиса. Для результатов YouTube Music видно,
  сколько раз проигрывали каждый трек, так что оригинал заметен среди
  каверов.
- **Одна очередь.** Смешивайте треки обоих сервисов, включайте
  перемешивание и повтор сразу для всех, а когда очередь закончится, пусть
  продолжает радио.
- **Explore (Обзор).** Новые релизы, популярные песни, чарты, настроения и
  жанры, а также похожие песни к тому, что играет.
- **Синхронизированные тексты песен,** которые идут за песней, или обычный
  текст, если вам так больше нравится.
- **Мини-плеер,** который остаётся поверх других окон.

### Ваша медиатека

- **Миксы:** плейлисты, в которых лежат треки из любого сервиса. Импортируйте
  плейлист сервиса в микс, а внутри любого из них ищите и сортируйте.
- **Понравившиеся песни** обоих сервисов в одном списке.
- **История и статистика прослушиваний** по дням.
- **Ваши учётные записи.** Войдите в YouTube Music на странице самого
  Google, чтобы получить домашнюю ленту, понравившиеся песни, плейлисты,
  альбомы и исполнителей, от имени вашей учётной записи или одного из её
  брендовых каналов. Войдите в Spotify ради вашей медиатеки там. Лайки,
  сохранённые альбомы и плейлисты и подписки на исполнителей доходят и до
  сервиса. То, что вы слушаете, попадает в вашу историю YouTube Music, по
  которой строятся его рекомендации.

### Spotify, тремя способами

- **Прямо в MixMusic,** в качестве Spotify, без запущенного приложения
  Spotify (Premium, на Windows и macOS).
- **Через Spotify Connect:** в вашем приложении Spotify, на телефоне или
  колонке.
- **Из YouTube Music:** MixMusic воспроизводит запись трека Spotify с
  YouTube Music, для чего Premium не нужен.

### Звук

- Кроссфейд, выравнивание громкости и нормализация уровня.
- Десятиполосный эквалайзер с пресетами.
- Визуализатор.
- Офлайн-кэш треков, которые вы слушали.

### На вашем рабочем столе

- Медиаклавиши и системные элементы управления мультимедиа в Windows, macOS
  и Linux.
- Свои глобальные сочетания клавиш: воспроизвести, поставить на паузу,
  переключить трек, изменить громкость, выключить звук, поставить лайк или
  дизлайк текущему треку или показать MixMusic, какое бы окно ни было на
  переднем плане.
- Кнопки плеера на миниатюре на панели задач в Windows.
- Управление из трея, уведомления о новых треках и запуск при входе в
  систему.
- Ссылки `mixmusic://`.
- **Last.fm:** каждый услышанный вами трек попадает в ваш профиль, а
  скробблы, сделанные офлайн, отправляются, как только вы снова онлайн.
- **Discord:** в вашем профиле видно, что играет, с обложкой и временем,
  которое осталось.
- **Обновления:** MixMusic проверяет, нет ли нового релиза, при запуске и
  каждые несколько часов, проверяет его подпись и ставит по вашей команде
  или сам, если вы так выбрали.

### Настройте под себя

- **Макеты.** Blend, макет по умолчанию, это скруглённые формы и упругая
  анимация на плотных рядах крупных обложек. Expressive, по мотивам Material 3
  Expressive, игривый: тональные заливки, кнопки-таблетки и плавающая панель
  плеера. Crate сдержанный, чтобы работали обложки, а Tiles это первый вид
  MixMusic, с плоскими плитками и гигантскими словами строчными буквами.
- **Темы.** Четырнадцать встроенных, каждая в любом макете, с эффектами,
  которые можно настроить, отдельной темой для светлого режима и для
  тёмного и акцентным цветом, своим или взятым из обложки играющего трека.
  Темы Material и Material Dark красят всё цветами обложки.
- **Темы и плагины сообщества** в маркетплейсе, в один клик.
- **Плагины** добавляют страницы, команды, действия над треками и виджеты
  через типизированный API, оформленные вашей темой.
- **В первую очередь клавиатура.** Сочетания клавиш для всего привычного,
  своя клавиша для любой команды, включая команды плагинов, клавиши,
  которые управляют музыкой из любого приложения, и палитра команд
  (<kbd>Ctrl</kbd> <kbd>K</kbd>) для остального.
- **Файл настроек.** Ваши настройки лежат в `settings.json` в папке данных
  (`%AppData%\MixMusic` в Windows, `~/Library/Application Support/MixMusic`
  в macOS, `~/.config/MixMusic` в Linux), и его можно хранить вместе с
  dotfiles. Сохраните его, пока MixMusic работает, и он применится сразу;
  Settings › Desktop › Settings file (Настройки › Рабочий стол › Файл настроек)
  открывает его.

## Взгляд изнутри

<p align="center">
  <img src="docs/look-listen.webp" alt="Страницы MixMusic в макете Blend, стоят прямо на тёмной сцене лицом к вам: в центре играет アイドル группы YOASOBI, обведённая розовым, под каждой строкой её текста ромадзи и перевод на английский; слева альбом THE BOOK 3, справа страница подкаста, на который вы подписаны" />
</p>

<p align="center"><sub><b>Слушайте.</b> В центре играет アイドル группы YOASOBI: каждая строка подсвечивается, пока её поют, а под ней ромадзи и перевод. Слева альбом, справа подкаст, на который вы подписаны.</sub></p>

<p align="center">
  <img src="docs/look-yours.webp" alt="Ещё MixMusic, стоит прямо на тёмной сцене лицом к вам: Settings › Appearance (Настройки › Оформление) с четырьмя макетами в центре, обведёнными розовым; слева тема Material, окрашенная обложкой играющего трека, справа темы сообщества в маркетплейсе" />
</p>

<p align="center"><sub><b>Настройте под себя.</b> В центре четыре макета: Blend, макет по умолчанию, Expressive, Crate и Tiles, под темами. Слева тема Material, которая берёт цвета из обложки играющего трека. Справа темы сообщества в маркетплейсе.</sub></p>

## Установка

Скачайте файл для вашей системы со страницы
[последнего релиза](https://github.com/fzn0x/mixmusic/releases/latest):

| Система | Загрузка |
|---|---|
| macOS 12 и новее (Apple silicon и Intel) | [`mixmusic-<version>-macos-universal.dmg`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Windows 10 и 11 (x64, Arm), установщик | [`mixmusic-<version>-windows-<arch>-setup.exe`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Windows, портативная версия, без установки | [`mixmusic-<version>-windows-<arch>-portable.zip`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Debian, Ubuntu и производные | [`mixmusic-<version>-linux-<arch>.deb`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Fedora, openSUSE и производные | [`mixmusic-<version>-linux-<arch>.rpm`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Arch Linux и производные (скоро) | [`mixmusic-bin`](https://aur.archlinux.org/packages/mixmusic-bin) в AUR: `yay -S mixmusic-bin` |
| Любой Linux, один файл | [`mixmusic-<version>-linux-<arch>.AppImage`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Любой Linux, в песочнице (Flatpak) | [`mixmusic-<version>-linux-<arch>.flatpak`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Любой Linux, в вашей домашней папке | [`mixmusic-<version>-linux-<arch>.tar.gz`](https://github.com/fzn0x/mixmusic/releases/latest), с `install.sh` |

`<arch>` это `amd64` для x64 и `arm64` для Arm. В каждом релизе есть
`SHA256SUMS`, по которому можно проверить загрузки. Файлы с именами
`update-…`, `….delta`, `mixmusic-<version>-macos-…` и
`-windows-<arch>.tar.gz` вам не нужны: из них обновляются уже установленные
приложения.

Установщики, портативный zip и архив для Linux обновляются сами. `.deb`,
`.rpm`, AppImage и Flatpak не могут заменить собственные файлы, поэтому
MixMusic сообщает, что вышла новая версия, и скачивает её для вас. `mixmusic-bin`
из AUR обновляется вместе с остальной системой. Портативный zip распакуйте
где угодно и откройте `MixMusic.exe`. AppImage сделайте исполняемым
(`chmod +x`) и откройте. Flatpak откройте через центр приложений или
выполните `flatpak install ./mixmusic-<version>-linux-<arch>.flatpak`; в
первый раз Flatpak скачает среду выполнения GNOME с Flathub.

Скоро, когда будет готов его tap: на Mac MixMusic ставит [Homebrew](https://brew.sh),
а `brew upgrade` его обновляет.

```sh
brew install --cask fzn0x/tap/mixmusic
```

### Первый запуск

- **macOS:** приложение ещё не нотаризовано Apple, поэтому при первом
  открытии macOS сообщает, что не может его проверить. Откройте его всё
  равно через System Settings › Privacy & Security › Open Anyway
  (Системные настройки › Конфиденциальность и безопасность › Всё равно
  открыть) или один раз выполните в Terminal (Терминале):

  ```sh
  xattr -dr com.apple.quarantine /Applications/MixMusic.app
  ```
- **Windows:** установщик ещё не подписан, поэтому SmartScreen может его
  предупредить: выберите More info (Подробнее), затем Run anyway (Выполнить
  в любом случае). Для входа в YouTube Music и воспроизведения Spotify в
  MixMusic нужен WebView2 Runtime, который есть в Windows 11.
- **Linux:** MixMusic нужны GTK 3 и PulseAudio (PipeWire тоже его
  обслуживает) или ALSA. Для входа в YouTube Music нужен WebKitGTK 4.1,
  для значка в трее нужен `libayatana-appindicator3`, а чтобы хранить ваши
  входы в системной связке ключей, нужен `libsecret`. `.deb` и `.rpm`
  ставят их, а Flatpak приносит их с собой; для AppImage и архива:

  ```sh
  # Debian and Ubuntu
  sudo apt install libgtk-3-0 libpulse0 libwebkit2gtk-4.1-0 libayatana-appindicator3-1 libsecret-1-0
  # Fedora
  sudo dnf install gtk3 pulseaudio-libs webkit2gtk4.1 libayatana-appindicator-gtk3 libsecret
  ```
- **Любая система:** звук большинства треков YouTube Music MixMusic находит
  сам. Для остальных, например треков, сделанных для детей, он использует
  [yt-dlp](https://github.com/yt-dlp/yt-dlp), который сам устанавливает
  из Settings › Playback (Настройки › Воспроизведение) и поддерживает в
  актуальном состоянии. yt-dlp нужна среда выполнения JavaScript:
  [Deno](https://deno.com), Node.js, Bun или QuickJS. В Flatpak есть
  Node.js.

### Flatpak

Flatpak запускает MixMusic в песочнице, которая видит сеть, звуковой
сервер, дисплей и GPU, но не видит вашу домашнюю папку. MixMusic хранит
настройки и медиатеку в `~/.var/app/io.github.fzn0x.mixmusic`, отдельно от
копии из `.deb`, `.rpm`, AppImage или архива, и обе могут работать
одновременно. Чтобы перенести настройки и медиатеку из такой копии,
закройте обе, скопируйте `settings.json`, `library.json`, `queue.json` и
папку `addons` из `~/.config/MixMusic` в
`~/.var/app/io.github.fzn0x.mixmusic/config/MixMusic` и войдите в свои
учётные записи заново: входы остаются в связке ключей другой копии.

Во Flatpak запуск при входе в систему идёт через ваш рабочий стол, который
может сначала спросить вас, а собственный yt-dlp должен лежать в
`~/.var/app/io.github.fzn0x.mixmusic`, поскольку другие папки песочнице
не видны. Приложение Discord, не установленное как Flatpak, показывает,
что играет, только если оно было запущено до MixMusic, а иначе до своего
перезапуска; Discord как Flatpak работает в любом случае. Flatpak новый:
расскажите нам в [issue](https://github.com/fzn0x/mixmusic/issues), как он
работает на вашем рабочем столе.

### Учётные записи (все необязательны)

- **YouTube Music:** выберите Sign in to YouTube Music (Войти в YouTube
  Music) в Settings › Accounts (Настройки › Учётные записи) и войдите в
  Google в открывшемся окне. Всё, что вы вводите, уходит только в Google.
  MixMusic хранит сеанс в зашифрованном виде, а ключ держит в системной
  связке ключей, где она есть. Если это окно не открывается или Google его
  отклоняет, войдите через свой браузер: MixMusic откроет его с новым
  профилем и удалит профиль, когда вы войдёте; импортируйте сеанс браузера,
  в котором вы вошли (MixMusic читает его через свой yt-dlp и оставляет
  только cookie YouTube); или вставьте заголовок Cookie вкладки
  music.youtube.com, где вы вошли.
- **Spotify:** Spotify позволяет приложению обслуживать до пяти
  пользователей и только пока у его владельца есть Premium, поэтому вы
  входите через собственное приложение Spotify. Это занимает две минуты:
  [docs/spotify.md](docs/spotify.md) проведёт вас по шагам.
- **Last.fm:** создайте учётную запись API на
  [last.fm/api/account/create](https://www.last.fm/api/account/create)
  (любое имя, без callback URL), вставьте её API key и shared secret в
  Settings › Accounts и подтвердите MixMusic в браузере.
- **Discord:** пока работает приложение Discord, в вашем профиле видно, что
  вы слушаете. Отключается в Settings › Desktop (Настройки › Рабочий стол).

Что-то не работает? [docs/troubleshooting.md](docs/troubleshooting.md)
описывает частые проблемы и известные неполадки.

## Сборка из исходников

Вам нужны [Go](https://go.dev/dl/) 1.27.3 или новее, [Bun](https://bun.sh)
1.2 или новее, на котором работает командная строка MyGo, и Git. MyGo не
нужен компилятор C, поэтому любая система собирает MixMusic для всех
систем.

### Windows

```powershell
winget install GoLang.Go
winget install Oven-sh.Bun
winget install Git.Git
```

Откройте новый терминал, затем:

```powershell
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

Для входа и воспроизведения Spotify нужен WebView2 Runtime, который есть в
Windows 11. В Windows 10 установите его с сайта
[Microsoft](https://developer.microsoft.com/microsoft-edge/webview2/).

### macOS

С [Homebrew](https://brew.sh):

```sh
brew install go oven-sh/bun/bun
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

### Linux

Установите Go с [go.dev/dl](https://go.dev/dl/) (пакеты дистрибутивов
часто старше 1.27.3), затем Bun и библиотеки, с которыми работает
MixMusic:

```sh
curl -fsSL https://bun.sh/install | bash
# Debian and Ubuntu
sudo apt install git libgtk-3-0 libpulse0 libwebkit2gtk-4.1-0 libayatana-appindicator3-1 libsecret-1-0
# Fedora
sudo dnf install git gtk3 pulseaudio-libs webkit2gtk4.1 libayatana-appindicator-gtk3 libsecret
```

Затем:

```sh
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

### Nix

С [Nix](https://nixos.org) команда `nix develop` даёт инструменты, которыми
собирают релизы, тех версий, что закреплены в `flake.lock`: Go, Bun, NSIS,
GoReleaser и Zola. `nix build` собирает MixMusic для Linux из исходников в
`result/`.

### Разработка

`bun run dev` собирает приложение, запускает его и пересобирает, пока вы
правите код. Его данные хранятся отдельно от установленного MixMusic, под
именем `MixMusic Dev`.

```sh
bun run build:plugins          # the plugins' JavaScript, from their TypeScript
go test ./...                  # the app's tests
bun run typecheck              # the plugin SDK and the community's plugins
go run ./tools/registry -check # the community registry
bun run build                  # the packaged app for this system, in build/
bun run build -- -platform windows/amd64,linux/amd64,linux/arm64
```

В оболочке Nix загрузки для Linux собирайте так:
`.github/scripts/build.sh linux/amd64,linux/arm64`, как это делают релизы.
Обычный `bun run build` там создаёт программу для Linux, которой нужен
собственный загрузчик Nix, и она запускается только на NixOS.
`go run ./tools/elfloader build/linux-amd64` проверяет сборку.

[Сайт](https://fzn0x.github.io/mixmusic/) лежит в `site/`, его собирает
[Zola](https://www.getzola.org), а руководства в `docs/` это его страницы
`/docs/`. `go run ./tools/sitedocs && zola --root site serve` показывает
его предварительно. Релизы собираются в оболочке Nix и публикуются
[GoReleaser](https://goreleaser.com) (`.goreleaser.yaml`).

[docs/architecture](docs/architecture/README.md) объясняет, как части
устроены вместе.

### Структура

Приложение это корневой пакет, а под ним пакеты из `internal/`. Пакеты вне
`internal/` предназначены для импорта кем угодно и описаны на
[pkg.go.dev](https://pkg.go.dev/github.com/fzn0x/mixmusic).

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

Пакеты, которые можно импортировать:

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

## Сообщество

Любой может установить темы и плагины сообщества из маркетплейса, значка
магазина на боковой панели, и добавить свои.

### Сделайте тему

Тема это файл `theme.json`, который задаёт цвета, шрифты и скругления,
видимые в макете, выбранном пользователем.
[docs/themes.md](docs/themes.md) перечисляет все токены и показывает
полную тему. Чтобы попробовать свою, положите её папку в папку тем
(Settings › Appearance › open themes folder, то есть Настройки › Оформление ›
открыть папку тем) и выберите её в Settings › Appearance.

### Сделайте плагин

Плагин это небольшой модуль на TypeScript или JavaScript, который
добавляет страницы, команды, действия над треками и виджеты.
[docs/plugins.md](docs/plugins.md) описывает API, а его
[типы](sdk/index.d.ts) помогут вашему редактору. Рабочие примеры из
сообщества: [таймер сна](community/plugins/sleep-timer) и
[спектр](community/plugins/spectrum).

### Поделитесь им

1. Сделайте форк этого репозитория.
2. Добавьте одну папку: `community/themes/<id>/` для темы или
   `community/plugins/<id>/` для плагина, с его `plugin.json` и его
   `index.ts`.
3. Выполните `bun run build:plugins`, `go run ./tools/registry` и
   `go run ./tools/registry -check`.
4. Откройте pull request.

После слияния CI публикует его в
[ветке registry](https://github.com/fzn0x/mixmusic/tree/registry), и он
появляется в маркетплейсе у всех. Подробности в
[community/README.md](community/README.md).

## Конфиденциальность

У MixMusic нет своих учётных записей, сервера, аналитики и телеметрии. Он
обращается к:

- **YouTube и Google:** каталог YouTube Music, его потоки и громкость
  каждого трека запрашиваются без входа, а также обложки; часть потоков
  через yt-dlp. После входа добавляется ваша медиатека там и, если вы не
  отключите это в Settings › Accounts (Настройки › Учётные записи),
  треки, которые вы слушаете и которые попадают в вашу историю YouTube
  Music.
- **Spotify,** после входа: ваша медиатека, ваши устройства Spotify
  Connect и плеер Spotify, который загружается с `sdk.scdn.co`, когда
  Spotify играет в MixMusic.
- **Unison:** ID видео играющего трека или, если его нет, название,
  исполнители, альбом и длительность, чтобы найти текст песни. И только
  пока вы включаете перевод или романизацию рядом с текстом: текст
  играющего трека вместе с ID его видео, который Unison отдаёт на перевод
  Google.
- **LRCLIB:** название, исполнители, альбом и длительность играющего
  трека, чтобы найти его текст.
- **GitHub:** проверка новой версии при запуске и каждые несколько часов,
  которую можно отключить в Settings › About (Настройки › О программе);
  реестр сообщества, когда вы открываете маркетплейс; загрузки yt-dlp; и,
  когда вы впервые включаете ромадзи рядом с японским текстом, словарь
  чтений кандзи (11,7 МБ). Ромадзи делаются на вашем компьютере: тексты
  не отправляются.
- **Last.fm,** после подключения: то, что вы слушаете.
- **Discord,** если вы не отключите: то, что играет, уходит в приложение
  Discord на вашем компьютере, которое показывает это в вашем профиле.

Что он хранит на вашем компьютере:

- Ваши настройки и медиатеку (понравившиеся и не понравившиеся песни,
  миксы, историю и недавние поисковые запросы) в папке, которую называет
  Settings › About.
- Треки, которые вы слушали, в кэше, в объёме, который вы разрешаете в
  Settings › Playback (Настройки › Воспроизведение), и громкость, которую
  измерил YouTube.
- Обложки, которые вы видели, до 256 МБ.
- Переводы текстов, которые вы включали, по трекам и языкам, и словарь, по
  которому ромадзи читает кандзи, после того как вы его включили.
- Ваши входы, включая сеанс YouTube Music, в системной связке ключей. В
  Windows, если Credential Manager не может сохранить вход, он попадает в
  файл, зашифрованный для вашей учётной записи Windows; в Linux без связки
  ключей это файлы, которые может читать только ваш пользователь.
- Журнал, без ваших входов.

Страница истории в медиатеке стирает то, что вы слушали, Settings ›
Playback очищает кэш и обложки, а выход из сервиса удаляет его вход.

В Windows визуализатор слышит звук Spotify, играющего на этом компьютере, в
MixMusic или в приложении Spotify, через process loopback Windows, который
слышит только это приложение. В macOS 14.2 и новее он слышит Spotify,
играющий в MixMusic, через process tap, когда вы это разрешите, как просит
macOS. Он анализирует звук в памяти и никогда не записывает и не
отправляет его.

## Благодарности

MixMusic построен на этих проектах:

- **[MyGo](https://github.com/egoist/mygo)**, фреймворк для десктопа, на
  котором построен MixMusic. MixMusic использует нативный UI MyGo: MyGo сам
  рисует каждое окно на Go, без webview, от страниц и панели плеера до
  мини-плеера. Его командная строка упаковывает приложение вместе с
  установщиками и подписанными обновлениями, из которых оно обновляется.
- **[Go](https://go.dev)**, язык приложения, вместе с
  [purego](https://github.com/ebitengine/purego), который вызывает
  системные библиотеки без cgo, а также
  [golang.org/x/image](https://pkg.go.dev/golang.org/x/image)
  и [golang.org/x/text](https://pkg.go.dev/golang.org/x/text).
- **[Sobek](https://github.com/grafana/sobek)**, движок JavaScript, в
  котором работает каждый плагин.
- **[Nix](https://nixos.org)**, который закрепляет инструменты, которыми
  собирают релизы, и собирает MixMusic для Linux из исходников.
- **[GoReleaser](https://goreleaser.com)**, который публикует релизы, и его
  [nFPM](https://nfpm.goreleaser.com), который делает пакеты RPM.
- **[Bun](https://bun.sh)**, который запускает сборку, и
  **[TypeScript](https://www.typescriptlang.org)**, который компилирует
  плагины и проверяет типы SDK плагинов.
- **[Zola](https://www.getzola.org)**, который собирает сайт.
- **[NSIS](https://nsis.sourceforge.io)**, который делает установщики для
  Windows, и [appimagetool](https://github.com/AppImage/appimagetool),
  который делает AppImage.
- **[yt-dlp](https://github.com/yt-dlp/yt-dlp)**, который находит звук
  треков, которые MixMusic не может найти сам.
- **[Unison](https://unison.betterlyrics.org)**, открытая база текстов
  песен, собираемая сообществом, под ODbL, и **[LRCLIB](https://lrclib.net)**,
  открытая база синхронизированных текстов песен.
- **[kagome](https://github.com/ikawaha/kagome)**, японский
  морфологический анализатор, который читает кандзи в текстах для их
  ромадзи, со словарём IPA из
  [kagome-dict](https://github.com/ikawaha/kagome-dict), сделанным из
  mecab-ipadic, который MixMusic скачивает, когда ромадзи включают в
  первый раз.
- **[libopus](https://opus-codec.org)**, портом чьего декодера является
  наш декодер Opus, и **[SharpJaad](https://github.com/jimm98y/SharpJaad)**,
  порт [JAAD](https://sourceforge.net/projects/jaadec/) на C#, от которого
  происходит декодер AAC.
- **[FFmpeg](https://ffmpeg.org)**, с которым сравниваются тесты
  декодеров.
- **[Selawik](https://github.com/microsoft/Selawik)**,
  **[Quicksand](https://github.com/andrew-paglinawan/QuicksandFamily)**,
  **[Roboto Flex](https://github.com/googlefonts/roboto-flex)** и
  **[Archivo](https://github.com/Omnibus-Type/Archivo)**, шрифты окон.
- **[Material Color Utilities](https://github.com/material-foundation/material-color-utilities)**,
  портом чьей цветовой науки HCT являются цвета тем Material.
- **[Kevin MacLeod](https://incompetech.com)**, чья музыка под лицензией
  [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/) играет на
  сайте.

## Участие в проекте

Исправления, новые функции, темы, плагины и сообщения об ошибках, всё
это приветствуется.

- **Сообщите о проблеме** через Settings › About › report a problem
  (Настройки › О программе › сообщить о проблеме). Откроется новый issue на
  GitHub с уже заполненными версией, системой, темой и плагинами. Или
  [откройте issue](https://github.com/fzn0x/mixmusic/issues/new) сами.
- **Отправьте изменение** как pull request. [CONTRIBUTING.md](CONTRIBUTING.md)
  объясняет, какие проверки запускать, какой стиль коммитов принят и какая
  запись в списке изменений нужна изменению, которое заметят люди.
- **Поделитесь темой или плагином:** см. [Сообщество](#сообщество).

[CONTRIBUTORS.md](CONTRIBUTORS.md) благодарит людей, чьи код, идеи и
сообщения сформировали MixMusic.

## Лицензия

MixMusic распространяется под двумя лицензиями: используйте его по
[лицензии MIT](LICENSE-MIT) или по [Стандартной общественной лицензии GNU,
версии 3](LICENSE-GPL), как вам больше нравится. Вклады принимаются под
обеими. Шрифты в `fonts` распространяются под SIL Open Font License:
[Selawik](fonts/OFL.txt), [Quicksand](fonts/quicksand-OFL.txt),
[Roboto Flex](fonts/robotoflex-OFL.txt) и
[Archivo](fonts/archivo-OFL.txt). Декодер Opus это порт libopus под
[её лицензией BSD](opus/LICENSE.libopus), декодер AAC происходит от
SharpJaad под [его лицензией MIT](aac/LICENSE.sharpjaad), а цветовая
наука HCT это порт Material Color Utilities под
[его лицензией Apache](internal/hct/LICENSE.material-color-utilities).

MixMusic не связан со Spotify и Google, не одобрен и не спонсируется ими.
Он воспроизводит то, что эти сервисы предлагают своим пользователям, через
их собственные интерфейсы; пользуйтесь им так, как позволяют их условия.
Spotify и YouTube Music это товарные знаки их владельцев.
