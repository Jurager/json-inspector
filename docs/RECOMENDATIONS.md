Рекомендации по стилю для проектов Google с открытым исходным кодом
===================================================================

Лучшие практики Go
------------------

Этот документ — часть документации по [стилю Go](https://google.github.io/styleguide/go/index) в Google. Он не является **ни** [**нормативным**](https://google.github.io/styleguide/go/index#normative)**, ни** [**каноничным**](https://google.github.io/styleguide/go/index#canonical), это дополнение к «[Руководству по стилю](https://google.github.io/styleguide/go/guide)». Подробности смотрите в [Обзоре](https://google.github.io/styleguide/go/index#about).

О документе
-----------

Здесь приведены рекомендации по лучшим практикам применения требований «Руководства по стилю» для Go. Это руководство охватывает общие и распространенные случаи, но не может применяться к каждому частному случаю. Обсуждение альтернатив, по возможности, включено в текст руководства вместе с указаниями о том, когда они применимы, а когда — нет.

Полная документация руководства по стилю описывается в [обзоре](https://google.github.io/styleguide/go/index#about).

Именование
----------

### Имена функций и методов

#### Избегайте повторений

При именовании функции или метода учитывайте контекст, в котором это имя читается. Чтобы не возникало лишних [повторений](https://google.github.io/styleguide/go/decisions#repetition) в точке вызова, следуйте рекомендациям ниже:

*   *   типы входных и выходных данных, если отсутствие указания не вызовет путаницу;

*   тип приемника метода;

*   является ли указателем входной или выходной элемент данных.

*   В функциях не [повторяйте имя пакета](https://google.github.io/styleguide/go/decisions#repetitive-with-package).


Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML `// Плохо: package yamlconfig func ParseYAMLConfig(input string) (*Config, error)Объяснить с`

Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML `// Хорошо: package yamlconfig func Parse(input string) (*Config, error)Объяснить с`

*   // Плохо:func (c \*Config) WriteConfigTo(w io.Writer) (int64, error)Объяснить с// Хорошо:func (c \*Config) WriteTo(w io.Writer) (int64, error)Объяснить с

*   // Плохо:func OverrideFirstWithSecond(dest, source \*Config) errorОбъяснить с// Хорошо:func Override(dest, source \*Config) errorОбъяснить с

*   // Плохо:func TransformYAMLToJSON(input \*Config) \*jsonconfig.ConfigОбъяснить с// Хорошо:func Transform(input \*Config) \*jsonconfig.ConfigОбъяснить с


Однако, если необходимо разграничить функции с похожим именем, допустимо включить в имя дополнительную информацию:

Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML`   // Хорошо:func (c *Config) WriteTextTo(w io.Writer) (int64, error)func (c *Config) WriteBinaryTo(w io.Writer) (int64, error)Объяснить с   `

#### Преобразование имен

Есть и другие правила именования методов и функций:

*   // Хорошо:func (c \*Config) JobName(key string) (value string, ok bool)Объяснить сТаким образом, префикса [Get](https://google.github.io/styleguide/go/decisions#getters) в именах функций и методов нужно избегать:// Плохо:func (c \*Config) GetJobName(key string) (value string, ok bool)Объяснить с

*   // Хорошо:func (c \*Config) WriteDetail(w io.Writer) (int64, error)Объяснить с

*   // Хорошо:func ParseInt(input string) (int, error)func ParseInt64(input string) (int64, error)func AppendInt(buf \[\]byte, value int) \[\]bytefunc AppendInt64(buf \[\]byte, value int64) \[\]byteОбъяснить сЕсли есть «первичная» версия, тип в ее имени можно опустить:// Хорошо:func (c \*Config) Marshal() (\[\]byte, error)func (c \*Config) MarshalText() (string, error)Объяснить сТестовые дубли пакетов и типов


При [именовании](https://google.github.io/styleguide/go/guide#naming) тестовых пакетов и типов, особенно [тестовых дублей (test doubles)](https://en.wikipedia.org/wiki/Test_double), применимо несколько правил. По своей функции тестовый дубль может быть заглушкой (stub), объектом-имитацией (fake), макетом объекта (mock) или тестовым шпионом (spy).

В данных примерах речь, как правило, идет о заглушках. Если в вашем случае это имитация или что-то другое, обновите имена соответствующим образом.

Допустим, у вас есть специализированный пакет, представляющий работающий код:

Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML`   package creditcardimport ( "errors" "path/to/money")// ErrDeclined указывает на то, что эмитент отклоняет платеж.var ErrDeclined = errors.New("creditcard: declined")// Карта содержит информацию о кредитной карте, такую ​​как ее эмитент,// срок действия и лимит.type Card struct {}// Сервис позволяет совершать операции с кредитными картами внешних// платежных систем, такие как взимание платы, авторизация, возмещение и подписка.type Service struct {}func (s *Service) Charge(c *Card, amount money.Money) error { /* опущено */ }Объяснить с   `

#### Создание вспомогательных тестовых пакетов (test helper packages)

Вы хотите создать пакет с тестовыми дублями для другого пакета. Воспользуемся выражением package creditcard, взятым из приведенного выше кода.

Вариант: можно ввести для теста новый пакет Go, создав его на основе работающего пакета. Чтобы не перепутать эти пакеты, к имени пакета припишем слово test: ("creditcard" + "test"):

Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML`   // Хорошо:package creditcardtestОбъяснить с   `

Если не указано иное, все примеры в последующих разделах будут писаться в рамках package creditcardtest.

#### Простой пример

Вы хотите добавить набор тестовых дублей для Service. Поскольку Card фактически заглушка, подобная сообщению Protocol Buffer, он не нуждается в специальной обработке в тестах, а значит, дублирование не требуется. Если вы ожидаете, что тестовые дубли будут применяться только для одного типа (например, Service), вы можете назвать дубли лаконично:

Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML`   // Хорошо:import ( "path/to/creditcard" "path/to/money")// Stub заглушает creditcard.Service и не предоставляет собственного поведения.type Stub struct{}func (Stub) Charge(*creditcard.Card, money.Money) error { return nil }Объяснить с   `

В отличии от имени StubService или, хуже того, StubCreditCardService, такой выбор имени открыто приветствуется, ведь имя базового пакета и типы его предметных областей дают достаточно информации о creditcardtest.Stub.

И наконец, если пакет создан в Bazel, убедитесь, что новое правило go\_library для этого пакета помечено как testonly:

Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML`   # Good:go_library( name = "creditcardtest", srcs = ["creditcardtest.go"], deps = [ ":creditcard", ":money", ], testonly = True,)Объяснить с   `

Это стандартный, вполне понятный большинству инженеров подход.

См. также:

*   [Совет № 42 по Go: Создание тестовой заглушки (Authoring a Stub for Testing)](https://google.github.io/styleguide/go/index.html#gotip)


#### Поведение нескольких тестовых дублей

Когда для ваших тестов требуется более одного варианта заглушек (например, нужна заглушка, которая всегда выдает ошибку), рекомендуется давать им имена, согласно моделируемому поведению. Например, Stub можно переименовать в AlwaysCharges и ввести новую заглушку — AlwaysDeclines:

Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML`   // Хорошо:// AlwaysCharges — заглушка creditcard.Service, симулирующая успех операции.type AlwaysCharges struct{}func (AlwaysCharges) Charge(*creditcard.Card, money.Money) error { return nil }// AlwaysDeclines — заглушка creditcard.Service, симулирующая отклонение платежа.type AlwaysDeclines struct{}func (AlwaysDeclines) Charge(*creditcard.Card, money.Money) error { return creditcard.ErrDeclined}Объяснить с   `

#### Несколько дублей для нескольких типов

Предположим, что package creditcard содержит несколько типов, и для каждого имеет смысл создавать дубли, как показано ниже для Service и StoredValue:

Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML`   package creditcardtype Service struct {}type Card struct {}// StoredValue управляет кредитными балансами клиентов. Структура // применима, когда возвращенный товар зачисляется на локальный счет // клиента, а не обрабатывается эмитентом кредита. По этой причине он// реализован как отдельный сервис.type StoredValue struct {}func (s *StoredValue) Credit(c *Card, amount money.Money) error { /* опущено */ }Объяснить с   `

В этом случае целесообразно давать тестовым дублям более явные имена:

Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML`   // Хорошо:type StubService struct{}func (StubService) Charge(*creditcard.Card, money.Money) error { return nil }type StubStoredValue struct{}func (StubStoredValue) Credit(*creditcard.Card, money.Money) error { return nil }Объяснить с   `

#### Локальные переменные в тестах

Если переменные относятся к тестовым дублям, их имена должны четко отличать дубли от работающих типов с учетом контекста. Допустим, вы хотите протестировать работающий код:

Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML`   package paymentimport ( "path/to/creditcard" "path/to/money")type CreditCard interface { Charge(*creditcard.Card, money.Money) error}type Processor struct { CC CreditCard}var ErrBadInstrument = errors.New("payment: instrument is invalid or expired")func (p *Processor) Process(c *creditcard.Card, amount money.Money) error { if c.Expired() { return ErrBadInstrument } return p.CC.Charge(c, amount)}Объяснить с   `

Тестовый дубль CreditCard с именем "spy" располагается рядом с рабочими типами, поэтому префикс перед именем поможет внести ясность:

Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML`   // Хорошо:package paymentimport "path/to/creditcardtest"func TestProcessor(t *testing.T) { var spyCC creditcardtest.Spy proc := &Processor{CC: spyCC} // объявления опущены: карта и сумма if err := proc.Process(card, amount); err != nil { t.Errorf("proc.Process(card, amount) = %v, want %v", got, want) } charges := []creditcardtest.Charge{ {Card: card, Amount: amount}, } if got, want := spyCC.Charges, charges; !cmp.Equal(got, want) { t.Errorf("spyCC.Charges = %v, want %v", got, want) }}Объяснить с   `

Так понятнее, чем без префикса.

Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML`   // Плохо:package paymentimport "path/to/creditcardtest"func TestProcessor(t *testing.T) { var cc creditcardtest.Spy proc := &Processor{CC: cc} // объявления опущены: карта и сумма if err := proc.Process(card, amount); err != nil { t.Errorf("proc.Process(card, amount) = %v, want %v", got, want) } charges := []creditcardtest.Charge{ {Card: card, Amount: amount}, } if got, want := cc.Charges, charges; !cmp.Equal(got, want) { t.Errorf("cc.Charges = %v, want %v", got, want) }}Объяснить с   `

### Затенение

> В этом разделе употребляются два неофициальных термина — это _сокрытие (stomping)_ и _затенение (shadowing)_. Они не относятся к официальной терминологии языка Go.

Как и во многих других языках, в Go есть изменяемые переменные. Это означает, что оператор присвоения меняет значение переменной.

Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML`   // Хорошо:func abs(i int) int { if i < 0 { i *= -1 } return i}Объяснить с   `

При [кратком объявлении переменных](https://go.dev/ref/spec#Short_variable_declarations) с помощью оператора := иногда новая переменная не создается. Мы называем это _сокрытием переменной (stomping)_. Оно вполне допустимо, когда начальное значение переменной нам больше не потребуется.

Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML`   // Хорошо:// innerHandler — хелпер для обработчика запросов, самостоятельно// отправляющий запросы другим бэкендам.func (s *Server) innerHandler(ctx context.Context, req *pb.MyRequest) *pb.MyResponse { // Unconditionally cap the deadline for this part of request handling. ctx, cancel := context.WithTimeout(ctx, 3*time.Second) defer cancel() ctxlog.Info("Capped deadline in inner request") // Код здесь больше не имеет доступа к исходному контексту. // Это хороший стиль, если при первом написании такого кода вы ожидаете, // что даже по мере роста кода ни одна корректная операция не должна // использовать (возможно, неограниченный) исходный контекст,  // предоставленный вызывающей стороной. // ...}Объяснить с   `

Но будьте осторожны с коротким объявлением переменных в новой области видимости. Оно приводит к созданию новой переменной. Мы называем это _затенением переменной_. Код после окончания блока относится к начальному значению. Ниже представлена ошибочная попытка сократить крайний срок выполнения (deadline) по условию:

Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML`   // Плохо:func (s *Server) innerHandler(ctx context.Context, req *pb.MyRequest) *pb.MyResponse { // Попытка ограничить срок условием. if *shortenDeadlines { ctx, cancel := context.WithTimeout(ctx, 3*time.Second) defer cancel() ctxlog.Info(ctx, "Capped deadline in inner request") } // БАГ: "ctx" здесь снова означает контекст, предоставленный  // вызывающей стороной. // Приведенный выше код с ошибками скомпилирован, потому что и ctx, и  // Cancel использовались внутри оператора if. // ...}Объяснить с   `

Корректный код может выглядеть так:

Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML`   // Хорошо:func (s *Server) innerHandler(ctx context.Context, req *pb.MyRequest) *pb.MyResponse { if *shortenDeadlines { var cancel func() // Применяется простое присвоение, = а не :=. ctx, cancel = context.WithTimeout(ctx, 3*time.Second) defer cancel() ctxlog.Info(ctx, "Capped deadline in inner request") } // ...}Объяснить с   `

Здесь мы скрыли (stomping) переменную. Поскольку новой переменной нет, назначаемый тип должен соответствовать типу начальной переменной. При затенении (shadowing) мы вводим полностью новый объект, который может иметь другой тип. Затенение может быть полезно, но здесь для [ясности](https://google.github.io/styleguide/go/guide#clarity) всегда используйте новое имя.

Использовать вне области видимости переменные с именами, которые повторяют имена исходных пакетов — не лучшая идея, ведь незадействованные функции такого пакета становятся недоступными. И наоборот, при выборе имени пакета избегайте имен, которые могут потребовать [переименования при импорте](https://google.github.io/styleguide/go/decisions#import-renaming) или затенить хорошие имена переменных на стороне клиента.

Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML`   // Плохо:func LongFunction() { url := "https://example.com/" // Oops, now we can't use net/url in code below.}Объяснить с   `

### Пакеты Util

Пакеты Go имеют имя, указанное в объявлении пакета, отдельно от пути импорта. Имя пакета для удобочитаемости важнее пути.

Имена пакетов Go должны быть [связаны с их функционалом](https://google.github.io/styleguide/go/decisions#package-names). Назвать пакет всего одним словом: util, helper, common или подобным образом, как правило, не лучшее решение (однако это слово может стать _частью_ имени). Неинформативные имена затрудняют чтение кода, а при частом использовании могут даже вызывать необоснованные [конфликты при импорте](https://google.github.io/styleguide/go/decisions#import-renaming).

Но точка вызова может выглядеть так:

Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML`   // Хорошо:db := spannertest.NewDatabaseFromFile(...)_, err := f.Seek(0, io.SeekStart)b := elliptic.Marshal(curve, x, y)Объяснить с   `

Приблизительное представление о функционале каждого объекта можно получить без списка импортов (cloud.google.com/go/spanner/spannertest, io и crypto/elliptic). С не столь содержательными именами код выглядел бы так:

Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML`   // Плохо:db := test.NewDatabaseFromFile(...)_, err := f.Seek(0, common.SeekStart)b := helper.Marshal(curve, x, y)Объяснить с   `

Размер пакета
-------------

Если вы задавались вопросом о том, насколько большим должен быть пакет в Go, и нужно ли помещать сходные типы в один пакет или разделять их по разным, поиск ответа стоит начать со статьи в [блога](https://go.dev/blog/package-names) об именах пакетов в Go. Статья посвящена не только именам. В ней есть полезные подсказки, обсуждения и цитаты по разным темам.

А вот некоторые другие соображения и замечания.

В качестве пакета на одной странице пользователи видят [godoc](https://pkg.go.dev/), а все методы, экспортируемые типами из пакета, группируются по их типу; godoc также группирует конструкторы вместе с возвращаемыми ими типами. Если _клиентскому коду_, скорее всего, потребуется взаимодействие двух значений разного типа, пользователю может быть удобно хранить их в одном пакете.

Код в рамках пакета может получить доступ к неэкспортированным идентификаторам внутри пакета. Если у вас есть несколько связанных типов, _реализация_ которых тесно связана, их размещение в одном пакете позволяет достичь связи между ними, не засоряя деталями об этой связи публичный API.

Тем не менее, если поместить весь проект в один пакет, такой пакет окажется непомерно раздутым. Когда часть проекта концептуально отличается от других частей, проще выделить аутентичную часть в отдельный пакет. Известное клиентам короткое имя пакета вместе с экспортируемым именем типа образуют понятный идентификатор, например bytes.Buffer, ring.New. В [этой](https://go.dev/blog/package-names) статье блога вы найдете больше примеров.

Стиль Go позволяет гибко менять размер файлов: при сопровождении пакета код можно перемещать внутри пакета из одного файла в другой без ущерба для вызывающих \[частей кода\]. Но, как показывает опыт, ни один файл со многими тысячами строк, ни множество маленьких файлов оптимальным решением не являются. В Go нет правила "один тип — один файл". Структура файлов организована достаточно хорошо, чтобы редактирующий его программист понимал, что и в каком файле искать. При этом файлы должны быть достаточно маленькими, чтобы в них было легче что-то найти. В стандартной библиотеке исходный код пакета часто разбивают на несколько файлов, группируя взаимосвязанный код в один файл. Хорошим примером может послужить код пакета [bytes](https://go.dev/src/bytes/). В пакетах с объемной сопроводительной документацией один doc.go можно выделить для [документации пакета](https://google.github.io/styleguide/go/decisions#package-comments) и его объявления. В общем случае включать туда что-то еще не требуется.

В кодовой базе Google и в проектах на Bazel расположение каталогов кода Go отличается от расположения кода в проектах Go с открытым исходным кодом: можно иметь несколько целевых объектов (targets) go\_library в одном каталоге. Если вы планируете сделать проект открытым, это хорошее обоснование, чтобы выделить каждому пакету отдельный каталог.

См. также:

*   [Пакеты тестовых дублей (Test double packages)](https://habr.com/ru/companies/skillfactory/articles/729924/#test-double-packages-and-types)


Импорты
-------

### Протоколы и заглушки

Импортирование библиотек протоколов отличается от импортирования других импортируемых объектов Go в плане обработки кросс-языковой спецификой. Правило для переименованных импортов proto основано на правиле создания пакета:

*   Суффикс pb обычно используется в рамках правил go\_proto\_library.

*   Суффикс grpc обычно используется в рамках правил go\_grpc\_library.


Префикс обычно состоит из одной или двух букв:

Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML`   // Хорошо:import ( fspb "path/to/package/foo_service_go_proto" fsgrpc "path/to/package/foo_service_go_grpc")Объяснить с   `

Если в пакете используется всего один протокол (proto) или пакет жестко привязан к протоколу, то префикс можно опустить:

Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML`   import ( pb "path/to/package/foo\_service\_go\_proto" grpc "path/to/package/foo\_service\_go\_grpc" )Объяснить с   `

Если в протоколе используются универсальные (generic) или малоинформативные символы, а также неочевидные сокращения, префиксом может стать короткое слово:

Plain textANTLR4BashCC#CSSCoffeeScriptCMakeDartDjangoDockerEJSErlangGitGoGraphQLGroovyHTMLJavaJavaScriptJSONJSXKotlinLaTeXLessLuaMakefileMarkdownMATLABMarkupObjective-CPerlPHPPowerShell.propertiesProtocol BuffersPythonRRubySass (Sass)Sass (Scss)SchemeSQLShellSwiftSVGTSXTypeScriptWebAssemblyYAMLXML`   // Хорошо:import ( mapspb "path/to/package/maps_go_proto")Объяснить с   `

Здесь, когда связь кода с картами неочевидна, mapspb.Address понять проще, чем mpb.Address.

### Порядок импорта

Как правило, импорты группируются в два и более блоков в такой последовательности:

1.  Стандартные библиотечные объекты, например "fmt".

2.  Другие импорты, например "/path/to/somelib".

3.  Опционально импорты протокольных буферов protobuf, например fpb "path/to/foo\_go\_proto".

4.  Опционально импорты побочных эффектов, например \_ "path/to/package".


Если файл не имеет группы для одной из указанных выше опциональных категорий, соответствующий иморт включается в группу импорта проекта.

Как правило, допустима любая ясная, доступная для понимания группировка импорта. Участники команды могут выбрать группировку импорта gRPC отдельно от импорта protobuf.

Для кода, содержащего только две обязательных группы, то есть стандартные библиотечные и другие импорты, инструмент goimports выдает результат, соответствующий требованиям этого руководства.

Однако об опциональных группах goimports ничего не знает, и поэтому аннулирует их. Если опциональные группы применяются, авторы и мейнтейнеры кода должны обратить внимание на соответствие группировки указанным требованиям.

При этом приемлем любой подход, предоставляющий полную и последовательную группировку в соответствии с указанными требованиями.

Это лишь небольшая часть документа, скоро мы опубликуем его продолжение. А пока вы можете начать практиковаться и получать полезный опыт на наших курсах:

*   [Профессия «Backend-разработчик на Go» (12 месяцев)](https://skillfactory.ru/backend-razrabotchik-na-golang?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=coding_go_200423&utm_term=conc)

*   [Профессия Fullstack-разработчик на Python (16 месяцев)](https://skillfactory.ru/python-fullstack-web-developer?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=coding_fpw_200423&utm_term=conc)


**Data Science и Machine Learning**

*   [Профессия Data Scientist](https://skillfactory.ru/data-scientist-pro?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=data-science_dspr_200423&utm_term=cat)

*   [Профессия Data Analyst](https://skillfactory.ru/data-analyst-pro?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=analytics_dapr_200423&utm_term=cat)

*   [Курс «Математика для Data Science»](https://skillfactory.ru/matematika-dlya-data-science#syllabus?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=data-science_mat_200423&utm_term=cat)

*   [Курс «Математика и Machine Learning для Data Science»](https://skillfactory.ru/matematika-i-machine-learning-dlya-data-science?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=data-science_matml_200423&utm_term=cat)

*   [Курс по Data Engineering](https://skillfactory.ru/data-engineer?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=data-science_dea_200423&utm_term=cat)

*   [Курс «Machine Learning и Deep Learning»](https://skillfactory.ru/machine-learning-i-deep-learning?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=data-science_mldl_200423&utm_term=cat)

*   [Курс по Machine Learning](https://skillfactory.ru/machine-learning?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=data-science_ml_200423&utm_term=cat)


**Python, веб-разработка**

*   [Профессия Fullstack-разработчик на Python](https://skillfactory.ru/python-fullstack-web-developer?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=coding_fpw_200423&utm_term=cat)

*   [Курс «Python для веб-разработки»](https://skillfactory.ru/python-for-web-developers?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=coding_pws_200423&utm_term=cat)

*   [Профессия Frontend-разработчик](https://skillfactory.ru/frontend-razrabotchik?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=coding_fr_200423&utm_term=cat)

*   [Профессия Веб-разработчик](https://skillfactory.ru/webdev?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=coding_webdev_200423&utm_term=cat)


**Мобильная разработка**

*   [Профессия iOS-разработчик](https://skillfactory.ru/ios-razrabotchik-s-nulya?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=coding_ios_200423&utm_term=cat)

*   [Профессия Android-разработчик](https://skillfactory.ru/android-razrabotchik?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=coding_andr_200423&utm_term=cat)


**Java и C#**

*   [Профессия Java-разработчик](https://skillfactory.ru/java-razrabotchik?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=coding_java_200423&utm_term=cat)

*   [Профессия QA-инженер на JAVA](https://skillfactory.ru/java-qa-engineer-testirovshik-po?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=coding_qaja_200423&utm_term=cat)

*   [Профессия C#-разработчик](https://skillfactory.ru/c-sharp-razrabotchik?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=coding_cdev_200423&utm_term=cat)

*   [Профессия Разработчик игр на Unity](https://skillfactory.ru/game-razrabotchik-na-unity-i-c-sharp?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=coding_gamedev_200423&utm_term=cat)


**От основ — в глубину**

*   [Курс «Алгоритмы и структуры данных»](https://skillfactory.ru/algoritmy-i-struktury-dannyh?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=coding_algo_200423&utm_term=cat)

*   [Профессия C++ разработчик](https://skillfactory.ru/c-plus-plus-razrabotchik?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=coding_cplus_200423&utm_term=cat)

*   [Профессия «Белый хакер»](https://skillfactory.ru/cyber-security-etichnij-haker?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=coding_hacker_200423&utm_term=cat)


**А также**

*   [Курс по DevOps](https://skillfactory.ru/devops-engineer?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=coding_devops_200423&utm_term=cat)

*   [Все курсы](https://skillfactory.ru/catalogue?utm_source=habr&utm_medium=habr&utm_campaign=article&utm_content=sf_allcourses_200423&utm_term=cat)


**Теги:**

*   [skillfactory](https://habr.com/ru/search/?target_type=posts&order=relevance&q=[skillfactory])

*   [go](https://habr.com/ru/search/?target_type=posts&order=relevance&q=[go])

*   [программирование](https://habr.com/ru/search/?target_type=posts&order=relevance&q=[программирование])

*   [кодирование](https://habr.com/ru/search/?target_type=posts&order=relevance&q=[кодирование])

*   [стиль](https://habr.com/ru/search/?target_type=posts&order=relevance&q=[стиль])

*   [рекомендации](https://habr.com/ru/search/?target_type=posts&order=relevance&q=[рекомендации])

*   [google](https://habr.com/ru/search/?target_type=posts&order=relevance&q=[google])

*   [руководство](https://habr.com/ru/search/?target_type=posts&order=relevance&q=[руководство])

*   [код](https://habr.com/ru/search/?target_type=posts&order=relevance&q=[код])

*   [детали](https://habr.com/ru/search/?target_type=posts&order=relevance&q=[детали])


**Хабы:**

*   [Блог компании Skillfactory](https://habr.com/ru/companies/skillfactory/articles/)

*   [Go](https://habr.com/ru/hubs/go/)

*   [Open source](https://habr.com/ru/hubs/open_source/)

*   [Программирование](https://habr.com/ru/hubs/programming/)