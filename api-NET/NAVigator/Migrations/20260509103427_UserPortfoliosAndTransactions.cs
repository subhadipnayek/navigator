using System;
using Microsoft.EntityFrameworkCore.Migrations;
using Npgsql.EntityFrameworkCore.PostgreSQL.Metadata;

#nullable disable

namespace NAVigator.Migrations
{
    /// <inheritdoc />
    public partial class UserPortfoliosAndTransactions : Migration
    {
        /// <inheritdoc />
        protected override void Up(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.CreateTable(
                name: "UserPortfolios",
                columns: table => new
                {
                    id = table.Column<long>(type: "bigint", nullable: false)
                        .Annotation("Npgsql:ValueGenerationStrategy", NpgsqlValueGenerationStrategy.IdentityByDefaultColumn),
                    user_id = table.Column<int>(type: "integer", nullable: false),
                    name = table.Column<string>(type: "character varying(120)", maxLength: 120, nullable: false),
                    created_at_utc = table.Column<DateTime>(type: "timestamp with time zone", nullable: false),
                    updated_at_utc = table.Column<DateTime>(type: "timestamp with time zone", nullable: false)
                },
                constraints: table =>
                {
                    table.PrimaryKey("pk_user_portfolios", x => x.id);
                    table.ForeignKey(
                        name: "fk_user_portfolios_app_users_user_id",
                        column: x => x.user_id,
                        principalTable: "AppUsers",
                        principalColumn: "id",
                        onDelete: ReferentialAction.Cascade);
                });

            migrationBuilder.CreateTable(
                name: "PortfolioTransactions",
                columns: table => new
                {
                    id = table.Column<long>(type: "bigint", nullable: false)
                        .Annotation("Npgsql:ValueGenerationStrategy", NpgsqlValueGenerationStrategy.IdentityByDefaultColumn),
                    portfolio_id = table.Column<long>(type: "bigint", nullable: false),
                    scheme_code = table.Column<int>(type: "integer", nullable: false),
                    transaction_date = table.Column<DateOnly>(type: "date", nullable: false),
                    transaction_type = table.Column<string>(type: "character varying(20)", maxLength: 20, nullable: false),
                    units = table.Column<decimal>(type: "numeric(18,6)", nullable: false),
                    nav = table.Column<decimal>(type: "numeric(18,4)", nullable: false),
                    amount = table.Column<decimal>(type: "numeric(18,2)", nullable: false),
                    notes = table.Column<string>(type: "character varying(240)", maxLength: 240, nullable: true),
                    created_at_utc = table.Column<DateTime>(type: "timestamp with time zone", nullable: false),
                    updated_at_utc = table.Column<DateTime>(type: "timestamp with time zone", nullable: false)
                },
                constraints: table =>
                {
                    table.PrimaryKey("pk_portfolio_transactions", x => x.id);
                    table.ForeignKey(
                        name: "fk_portfolio_transactions_funds_scheme_code",
                        column: x => x.scheme_code,
                        principalTable: "Funds",
                        principalColumn: "scheme_code",
                        onDelete: ReferentialAction.Restrict);
                    table.ForeignKey(
                        name: "fk_portfolio_transactions_user_portfolios_portfolio_id",
                        column: x => x.portfolio_id,
                        principalTable: "UserPortfolios",
                        principalColumn: "id",
                        onDelete: ReferentialAction.Cascade);
                });

            migrationBuilder.CreateIndex(
                name: "ix_portfolio_transactions_portfolio_id_scheme_code_transaction_",
                table: "PortfolioTransactions",
                columns: new[] { "portfolio_id", "scheme_code", "transaction_date" });

            migrationBuilder.CreateIndex(
                name: "ix_portfolio_transactions_portfolio_id_transaction_date",
                table: "PortfolioTransactions",
                columns: new[] { "portfolio_id", "transaction_date" });

            migrationBuilder.CreateIndex(
                name: "ix_portfolio_transactions_scheme_code",
                table: "PortfolioTransactions",
                column: "scheme_code");

            migrationBuilder.CreateIndex(
                name: "ix_user_portfolios_user_id_name",
                table: "UserPortfolios",
                columns: new[] { "user_id", "name" },
                unique: true);
        }

        /// <inheritdoc />
        protected override void Down(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.DropTable(
                name: "PortfolioTransactions");

            migrationBuilder.DropTable(
                name: "UserPortfolios");
        }
    }
}
