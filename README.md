# qpack — .dat -> .qpac

![qpack icon](assets/icon.png)

Программа для сжатия сырых данных в архивный формат.
При работе удаляются нулевые и мусорные строки.


## Запуск

### Через исполняемый файл
```
# .dat -> .qpac
./qpack pack <inDir> <outDir> [--run <name>] [--config <cfg.yaml>] [--tar] 

# .qpac -> .dat
./qpack unpack <inDir|in.tar.gz> <outDir> [--config <cfg.yaml>] 

# записать встроенный конфиг (по умолч. default.yaml)
./qpack init-config [path]
``` 

### Через Go
```
# .dat -> .qpac
go run ./cmd pack <inDir> <outDir> [--run <name>] [--config <cfg.yaml>] [--tar]

# .qpac -> .dat
go run ./cmd unpack <inDir|in.tar.gz> <outDir> [--config <cfg.yaml>]

# записать встроенный конфиг (по умолч. default.yaml)
go run ./cmd init-config [path]
```

`pack:` берёт все `.dat` из `<inDir>`, создаёт `<outDir>/<name>/` (по умолчанию `QPack_inDir`):

Внутри`.qpac` файлы, `detector.qpack.yaml` и `fingerprint.txt`.
`--tar`: создает архив `<outDir>/<name>.tar.gz`.

`unpack:` разворачивает `.qpac` обратно в `.dat` построчно в исходном формате

Ось (`_X_`/`_Y_`) определяется по имени входного файла.

## Конфиг

Порядок выбора: `--config <cfg.yaml>` → `default.yaml` рядом → вопрос
`Warning! No config found. Continue with default config? (y/n)` (отказ/EOF = fail-closed, exit 1).

## Логи

Мусорные/нулевые строки, ошибки пишутся `SKIP/EMPTY/ERROR` в `.log`.

## Формат qpac (v12)

Заголовок 16 байт: `magic(0x4E4F554D), version(12), layers(3), plates(6)`.
Событие бит-пак: `[marker:5][tAbs:64][tdelta:25][(1+idx:5/cnt-1:5/kb:5)*,0]`.

## Зависимости

`go 1.27.1`,
`gopkg.in/yaml.v3`.
