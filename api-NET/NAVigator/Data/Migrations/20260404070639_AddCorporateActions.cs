using System;
using Microsoft.EntityFrameworkCore.Migrations;

#nullable disable

namespace NAVigator.Data.Migrations
{
    /// <inheritdoc />
    public partial class AddCorporateActions : Migration
    {
        /// <inheritdoc />
        protected override void Up(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.CreateTable(
                name: "CorporateActions",
                columns: table => new
                {
                    Id = table.Column<int>(type: "int", nullable: false)
                        .Annotation("SqlServer:Identity", "1, 1"),
                    SchemeCode = table.Column<int>(type: "int", nullable: false),
                    ActionDate = table.Column<DateOnly>(type: "date", nullable: false),
                    ActionType = table.Column<string>(type: "nvarchar(50)", maxLength: 50, nullable: false),
                    AdjustmentFactor = table.Column<decimal>(type: "decimal(18,10)", nullable: false),
                    OldFaceValue = table.Column<decimal>(type: "decimal(18,6)", nullable: true),
                    NewFaceValue = table.Column<decimal>(type: "decimal(18,6)", nullable: true),
                    Notes = table.Column<string>(type: "nvarchar(500)", maxLength: 500, nullable: true),
                    CreatedAtUtc = table.Column<DateTime>(type: "datetime2", nullable: false)
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

            migrationBuilder.CreateIndex(
                name: "IX_CorporateActions_SchemeCode",
                table: "CorporateActions",
                column: "SchemeCode");

            migrationBuilder.CreateIndex(
                name: "IX_CorporateActions_SchemeCode_ActionDate",
                table: "CorporateActions",
                columns: new[] { "SchemeCode", "ActionDate" });
        }

        /// <inheritdoc />
        protected override void Down(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.DropTable(
                name: "CorporateActions");
        }
    }
}
