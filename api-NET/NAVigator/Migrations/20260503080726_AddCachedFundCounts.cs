using Microsoft.EntityFrameworkCore.Migrations;

#nullable disable

namespace NAVigator.Migrations
{
    /// <inheritdoc />
    public partial class AddCachedFundCounts : Migration
    {
        /// <inheritdoc />
        protected override void Up(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.AddColumn<int>(
                name: "corporate_action_count",
                table: "Funds",
                type: "integer",
                nullable: false,
                defaultValue: 0);

            migrationBuilder.AddColumn<int>(
                name: "nav_history_count",
                table: "Funds",
                type: "integer",
                nullable: false,
                defaultValue: 0);

            // Backfill cached counts from existing data
            migrationBuilder.Sql(@"
                UPDATE ""Funds"" f SET
                    nav_history_count = (SELECT COUNT(*) FROM ""NavHistories"" n WHERE n.scheme_code = f.scheme_code),
                    corporate_action_count = (SELECT COUNT(*) FROM ""CorporateActions"" ca WHERE ca.scheme_code = f.scheme_code);
            ");
        }

        /// <inheritdoc />
        protected override void Down(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.DropColumn(
                name: "corporate_action_count",
                table: "Funds");

            migrationBuilder.DropColumn(
                name: "nav_history_count",
                table: "Funds");
        }
    }
}
