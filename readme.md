# minpack — .dat -> архивный min (.min)

Микросервис сжатия: больше ничего не делает (ни сшивки, ни заливки, ни дампов).
Скинуть папку на другой комп, сжать файлы, `.min` забрать обратно.
## Запуск

```
go run ./cmd pack <inDir> <outDir> [--run <name>] [--config <path.yaml>] [--tar]
go run ./cmd unpack <inDir> <outDir> [--config <path.yaml>]  # .min -> .dat (см. ниже)
go run ./cmd init-config [path]                                  # записать встроенный конфиг (по умолч. default.yaml)
```

pack: берёт все `.dat` из `<inDir>`, создаёт `<outDir>/<name>/` (по умолчанию `run_YYYYMMDD_HHMMSS):
`.min` файлы (структура подпапок сохраняется), `detector.minpack.yaml` + `fingerprint.txt` рядом.
`--tar`: только архив `<outDir>/<name>.tar.gz`, папка рядом не создаётся (стейджинг в tmp, внутри архива run-папка `<name>/`).
unpack: разворачивает `.min` обратно в `.dat` построчно в исходном формате
(`trig n delta t_abs t_sec <22 hex> <10 нулей>`); номер события `n` — заново с 1
(в min не хранится), разделители и place-хвост — нулями (в min не хранятся).
Ось (`_X_`/`_Y_`) определяется по имени входного файла.
Мусорные строки (ненулевые разделители) скипаются с `SKIP` в лог, в файл не идут.

## Конфиг

Порядок выбора: `--config <файл>` → `default.yaml` рядом (cwd) → вопрос
`Write built-in default as ./default.yaml? (y/n)` (отказ/EOF = fail-closed, exit 1).

## Логи

всё — в `log/pack.log` (`START / DONE / ERROR / SKIP`, append, ротации нет);
в консоль — только ошибки (`error: ...`) и финальный результат (`pack ... -> ... files=N`).
Папка `log/` в git не идёт.

## Формат min (v12)

Шапка 16 байт LE: `magic(0x4E4F554D), version(12), layers(3), plates(6)`.
Событие бит-пак LSB-first: `[marker:3][tLen:4][tAbs|tDelta][delta:25][(1+idx:5/cnt-1:5/kb:5)*,0]`.
Первое событие хранит абсолютное время `t` в `tdelta_bits` бит (tLen=0);
дальше дельты `t-prev` varint: длина tLen бит (минимум 1), значение без старшего бита следом.
Пустые события (все платы нулевые) скипаются, в файл не идут.
Всё кроме номера события `n` (col1), сепараторов и place-хвоста.
Параметры — `internal/config/default.yaml` (`tdelta_bits: 35`, `tdelta_vbits: 5`).

## Зависимости

Только `gopkg.in/yaml.v3` (конфиг). Снаружи нужен лишь Go.
