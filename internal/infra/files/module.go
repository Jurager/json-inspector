package files

import "go.uber.org/fx"

var Module = fx.Module("infra/files", fx.Provide(NewReader))
