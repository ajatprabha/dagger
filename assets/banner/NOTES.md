# dagger banner

| File | Purpose |
| --- | --- |
| `gen.py` | Generates the SVG. Edit this file, not the SVG. |
| `dagger-banner.svg` | The source image. |
| `dagger-banner.png` | The image that the README shows. |

## What it shows

The banner is a trace view, in the style of Jaeger or Grafana Tempo, of one dagger workflow.
The workflow attaches the custom domain `shop.example`.
The scenario comes from the post [What an error means in your Go application](https://ajatprabha.in/2026/10/13/application-error-handling).

- Steps run in series.
- The first `checkCNAME` attempt fails because the CNAME record is missing. The error kind is transient, so a retry middleware runs the step again.
- `app:parallel` is a custom Step type that runs three steps at the same time. dagger does not supply it. The README shows how to write it.
- `issueCertificate` fails with a rate-limit error. `Result` and `Switch` route the error to `scheduleRequeue`.

Each element in the image must match something that the library or the README supports.
If you change the scenario, run the workflow first and make sure that it behaves as the image shows.

## Why a PNG

The SVG uses `<text>`.
GitHub shows README images through a proxy, and the viewer's browser supplies the fonts.
A PNG looks the same everywhere.

## Regenerate

You need Python 3, [resvg](https://github.com/linebender/resvg) (`cargo install resvg`), [Inter](https://github.com/rsms/inter/releases), and [JetBrains Mono](https://github.com/JetBrains/JetBrainsMono/releases).
Set `INTER` and `JBM` to the directories that contain the `.ttf` files.

```bash
cd assets/banner
python3 gen.py > dagger-banner.svg
resvg --skip-system-fonts \
  --use-font-file "$INTER/Inter-Regular.ttf" \
  --use-font-file "$INTER/Inter-SemiBold.ttf" \
  --use-font-file "$INTER/Inter-Bold.ttf" \
  --use-font-file "$INTER/InterDisplay-Bold.ttf" \
  --use-font-file "$JBM/JetBrainsMono-Regular.ttf" \
  --use-font-file "$JBM/JetBrainsMono-Bold.ttf" \
  -w 2400 dagger-banner.svg dagger-banner.png
```

The PNG is 2400 pixels wide, two times the SVG width, so it stays sharp on high-density screens.

## Palette

| Role | Hex |
| --- | --- |
| Background | `#0B1220` |
| Panel | `#0F1A2C` |
| Step succeeded | `#4F8CFF` |
| Transient error, retried | `#F5A524` |
| Error, routed | `#F25F5C` |
| Custom Step | `#A78BFA` |
| dagger control flow | `#2A3A55` |

The background is dark, with rounded and transparent corners.
It looks correct on the light and dark GitHub themes.
