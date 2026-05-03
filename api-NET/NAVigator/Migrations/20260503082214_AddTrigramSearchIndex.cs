using Microsoft.EntityFrameworkCore.Migrations;

#nullable disable

namespace NAVigator.Migrations
{
    /// <inheritdoc />
    public partial class AddTrigramSearchIndex : Migration
    {
        /// <inheritdoc />
        protected override void Up(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.AlterDatabase()
                .Annotation("Npgsql:PostgresExtension:pg_trgm", ",,");

            migrationBuilder.CreateIndex(
                name: "ix_funds_fund_house",
                table: "Funds",
                column: "fund_house")
                .Annotation("Npgsql:IndexMethod", "gin")
                .Annotation("Npgsql:IndexOperators", new[] { "gin_trgm_ops" });

            migrationBuilder.CreateIndex(
                name: "ix_funds_scheme_name",
                table: "Funds",
                column: "scheme_name")
                .Annotation("Npgsql:IndexMethod", "gin")
                .Annotation("Npgsql:IndexOperators", new[] { "gin_trgm_ops" });
        }

        /// <inheritdoc />
        protected override void Down(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.DropIndex(
                name: "ix_funds_fund_house",
                table: "Funds");

            migrationBuilder.DropIndex(
                name: "ix_funds_scheme_name",
                table: "Funds");

            migrationBuilder.AlterDatabase()
                .OldAnnotation("Npgsql:PostgresExtension:pg_trgm", ",,");
        }
    }
}
