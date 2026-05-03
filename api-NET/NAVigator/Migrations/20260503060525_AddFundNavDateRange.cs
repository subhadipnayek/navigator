using System;
using Microsoft.EntityFrameworkCore.Migrations;

#nullable disable

namespace NAVigator.Migrations
{
    /// <inheritdoc />
    public partial class AddFundNavDateRange : Migration
    {
        /// <inheritdoc />
        protected override void Up(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.AddColumn<DateOnly>(
                name: "nav_end_date",
                table: "Funds",
                type: "date",
                nullable: true);

            migrationBuilder.AddColumn<DateOnly>(
                name: "nav_start_date",
                table: "Funds",
                type: "date",
                nullable: true);

            migrationBuilder.Sql("""
                UPDATE "Funds" AS f
                SET
                    nav_start_date = ranges.start_date,
                    nav_end_date = ranges.end_date
                FROM (
                    SELECT scheme_code, MIN(date) AS start_date, MAX(date) AS end_date
                    FROM "NavHistories"
                    GROUP BY scheme_code
                ) AS ranges
                WHERE f.scheme_code = ranges.scheme_code;
                """);
        }

        /// <inheritdoc />
        protected override void Down(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.DropColumn(
                name: "nav_end_date",
                table: "Funds");

            migrationBuilder.DropColumn(
                name: "nav_start_date",
                table: "Funds");
        }
    }
}
