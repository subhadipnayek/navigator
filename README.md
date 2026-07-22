# NAVigator welcome
AVigator is a .NET 10 / ASP.NET Core Web API for tracking mutual fund NAV (Net Asset Value) data, managing investor portfolios, and analyzing fund performance.
Key Features:
•	📈 NAV Sync & History — Automated syncing of daily NAV data from AMFI (Association of Mutual Funds in India), with gap detection and anomaly tracking (NavSyncWorker, AmfiService, NavHistoryProvider)
•	💼 Portfolio Management — Track user portfolios, transactions, and dashboard caching, with Excel import support (ExcelTransactionWorkbookParser)
•	📊 Simulation & Analytics — Portfolio simulation, what-if analysis, rolling returns, and XIRR (Extended Internal Rate of Return) calculations
•	🏢 Corporate Actions — Track and review corporate actions (splits, dividends, mergers) affecting fund NAVs
•	🔐 Authentication — JWT-based auth with PBKDF2 password hashing
•	🗄️ Data Layer — EF Core with PostgreSQL (Npgsql), code-first migrations
•	⚙️ Admin Tools — Admin controller for managing sync operations and data review queues
Tech Stack: ASP.NET Core (.NET 10), Entity Framework Core, PostgreSQL, JWT Bearer Auth, Polly (resiliency)
Suggested GitHub Topics/Tags: dotnet, aspnetcore, mutual-funds, nav, portfolio-management, entity-framework-core, postgresql, finance, xirr, amfi
