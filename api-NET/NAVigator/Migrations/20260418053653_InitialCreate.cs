using System;
using Microsoft.EntityFrameworkCore.Migrations;
using Npgsql.EntityFrameworkCore.PostgreSQL.Metadata;

#nullable disable

namespace NAVigator.Migrations
{
    /// <inheritdoc />
    public partial class InitialCreate : Migration
    {
        /// <inheritdoc />
        protected override void Up(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.CreateTable(
                name: "Funds",
                columns: table => new
                {
                    SchemeCode = table.Column<int>(type: "integer", nullable: false),
                    IsinGrowth = table.Column<string>(type: "character varying(20)", maxLength: 20, nullable: true),
                    IsinDivReinvestment = table.Column<string>(type: "character varying(20)", maxLength: 20, nullable: true),
                    SchemeName = table.Column<string>(type: "character varying(500)", maxLength: 500, nullable: false),
                    LatestNav = table.Column<decimal>(type: "numeric(18,6)", nullable: true),
                    LatestNavDate = table.Column<DateOnly>(type: "date", nullable: true),
                    FundHouse = table.Column<string>(type: "character varying(300)", maxLength: 300, nullable: true),
                    SchemeType = table.Column<string>(type: "character varying(200)", maxLength: 200, nullable: true),
                    SchemeCategory = table.Column<string>(type: "character varying(300)", maxLength: 300, nullable: true),
                    CreatedAt = table.Column<DateTime>(type: "timestamp with time zone", nullable: false),
                    UpdatedAt = table.Column<DateTime>(type: "timestamp with time zone", nullable: false)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_Funds", x => x.SchemeCode);
                });

            migrationBuilder.CreateTable(
                name: "CorporateActions",
                columns: table => new
                {
                    Id = table.Column<int>(type: "integer", nullable: false)
                        .Annotation("Npgsql:ValueGenerationStrategy", NpgsqlValueGenerationStrategy.IdentityByDefaultColumn),
                    SchemeCode = table.Column<int>(type: "integer", nullable: false),
                    ActionDate = table.Column<DateOnly>(type: "date", nullable: false),
                    ActionType = table.Column<string>(type: "character varying(50)", maxLength: 50, nullable: false),
                    AdjustmentFactor = table.Column<decimal>(type: "numeric(18,10)", nullable: false),
                    OldFaceValue = table.Column<decimal>(type: "numeric(18,6)", nullable: true),
                    NewFaceValue = table.Column<decimal>(type: "numeric(18,6)", nullable: true),
                    Notes = table.Column<string>(type: "character varying(500)", maxLength: 500, nullable: true),
                    CreatedAtUtc = table.Column<DateTime>(type: "timestamp with time zone", nullable: false)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_CorporateActions", x => x.Id);
                    table.ForeignKey(
                        name: "FK_CorporateActions_Funds_SchemeCode",
                        column: x => x.SchemeCode,
                        principalTable: "Funds",
                        principalColumn: "SchemeCode",
                        onDelete: ReferentialAction.Cascade);
                });

            migrationBuilder.CreateTable(
                name: "NavHistories",
                columns: table => new
                {
                    Id = table.Column<long>(type: "bigint", nullable: false)
                        .Annotation("Npgsql:ValueGenerationStrategy", NpgsqlValueGenerationStrategy.IdentityByDefaultColumn),
                    SchemeCode = table.Column<int>(type: "integer", nullable: false),
                    Date = table.Column<DateOnly>(type: "date", nullable: false),
                    Nav = table.Column<decimal>(type: "numeric(18,6)", nullable: false)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_NavHistories", x => x.Id);
                    table.ForeignKey(
                        name: "FK_NavHistories_Funds_SchemeCode",
                        column: x => x.SchemeCode,
                        principalTable: "Funds",
                        principalColumn: "SchemeCode",
                        onDelete: ReferentialAction.Cascade);
                });

            migrationBuilder.CreateTable(
                name: "NavSyncTrackers",
                columns: table => new
                {
                    SchemeCode = table.Column<int>(type: "integer", nullable: false),
                    LastApiCheckUtc = table.Column<DateTime>(type: "timestamp with time zone", nullable: false),
                    LatestNavDateFromApi = table.Column<DateOnly>(type: "date", nullable: true),
                    LastCheckHadNewData = table.Column<bool>(type: "boolean", nullable: false)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_NavSyncTrackers", x => x.SchemeCode);
                    table.ForeignKey(
                        name: "FK_NavSyncTrackers_Funds_SchemeCode",
                        column: x => x.SchemeCode,
                        principalTable: "Funds",
                        principalColumn: "SchemeCode",
                        onDelete: ReferentialAction.Cascade);
                });

            migrationBuilder.CreateIndex(
                name: "IX_CorporateActions_SchemeCode",
                table: "CorporateActions",
                column: "SchemeCode");

            migrationBuilder.CreateIndex(
                name: "IX_CorporateActions_SchemeCode_ActionDate",
                table: "CorporateActions",
                columns: new[] { "SchemeCode", "ActionDate" });

            migrationBuilder.CreateIndex(
                name: "IX_NavHistories_SchemeCode",
                table: "NavHistories",
                column: "SchemeCode");

            migrationBuilder.CreateIndex(
                name: "IX_NavHistories_SchemeCode_Date",
                table: "NavHistories",
                columns: new[] { "SchemeCode", "Date" },
                unique: true);
        }

        /// <inheritdoc />
        protected override void Down(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.DropTable(
                name: "CorporateActions");

            migrationBuilder.DropTable(
                name: "NavHistories");

            migrationBuilder.DropTable(
                name: "NavSyncTrackers");

            migrationBuilder.DropTable(
                name: "Funds");
        }
    }
}
