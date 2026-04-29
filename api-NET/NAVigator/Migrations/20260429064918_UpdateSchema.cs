using Microsoft.EntityFrameworkCore.Migrations;

#nullable disable

namespace NAVigator.Migrations
{
    /// <inheritdoc />
    public partial class UpdateSchema : Migration
    {
        /// <inheritdoc />
        protected override void Up(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.DropForeignKey(
                name: "FK_CorporateActions_Funds_SchemeCode",
                table: "CorporateActions");

            migrationBuilder.DropForeignKey(
                name: "FK_NavHistories_Funds_SchemeCode",
                table: "NavHistories");

            migrationBuilder.DropForeignKey(
                name: "FK_NavSyncTrackers_Funds_SchemeCode",
                table: "NavSyncTrackers");

            migrationBuilder.DropPrimaryKey(
                name: "PK_NavSyncTrackers",
                table: "NavSyncTrackers");

            migrationBuilder.DropPrimaryKey(
                name: "PK_NavHistories",
                table: "NavHistories");

            migrationBuilder.DropPrimaryKey(
                name: "PK_Funds",
                table: "Funds");

            migrationBuilder.DropPrimaryKey(
                name: "PK_CorporateActions",
                table: "CorporateActions");

            migrationBuilder.RenameColumn(
                name: "LatestNavDateFromApi",
                table: "NavSyncTrackers",
                newName: "latest_nav_date_from_api");

            migrationBuilder.RenameColumn(
                name: "LastCheckHadNewData",
                table: "NavSyncTrackers",
                newName: "last_check_had_new_data");

            migrationBuilder.RenameColumn(
                name: "LastApiCheckUtc",
                table: "NavSyncTrackers",
                newName: "last_api_check_utc");

            migrationBuilder.RenameColumn(
                name: "SchemeCode",
                table: "NavSyncTrackers",
                newName: "scheme_code");

            migrationBuilder.RenameColumn(
                name: "Nav",
                table: "NavHistories",
                newName: "nav");

            migrationBuilder.RenameColumn(
                name: "Date",
                table: "NavHistories",
                newName: "date");

            migrationBuilder.RenameColumn(
                name: "Id",
                table: "NavHistories",
                newName: "id");

            migrationBuilder.RenameColumn(
                name: "SchemeCode",
                table: "NavHistories",
                newName: "scheme_code");

            migrationBuilder.RenameIndex(
                name: "IX_NavHistories_SchemeCode_Date",
                table: "NavHistories",
                newName: "ix_nav_histories_scheme_code_date");

            migrationBuilder.RenameIndex(
                name: "IX_NavHistories_SchemeCode",
                table: "NavHistories",
                newName: "ix_nav_histories_scheme_code");

            migrationBuilder.RenameColumn(
                name: "UpdatedAt",
                table: "Funds",
                newName: "updated_at");

            migrationBuilder.RenameColumn(
                name: "SchemeType",
                table: "Funds",
                newName: "scheme_type");

            migrationBuilder.RenameColumn(
                name: "SchemeName",
                table: "Funds",
                newName: "scheme_name");

            migrationBuilder.RenameColumn(
                name: "SchemeCategory",
                table: "Funds",
                newName: "scheme_category");

            migrationBuilder.RenameColumn(
                name: "LatestNavDate",
                table: "Funds",
                newName: "latest_nav_date");

            migrationBuilder.RenameColumn(
                name: "LatestNav",
                table: "Funds",
                newName: "latest_nav");

            migrationBuilder.RenameColumn(
                name: "IsinGrowth",
                table: "Funds",
                newName: "isin_growth");

            migrationBuilder.RenameColumn(
                name: "IsinDivReinvestment",
                table: "Funds",
                newName: "isin_div_reinvestment");

            migrationBuilder.RenameColumn(
                name: "FundHouse",
                table: "Funds",
                newName: "fund_house");

            migrationBuilder.RenameColumn(
                name: "CreatedAt",
                table: "Funds",
                newName: "created_at");

            migrationBuilder.RenameColumn(
                name: "SchemeCode",
                table: "Funds",
                newName: "scheme_code");

            migrationBuilder.RenameColumn(
                name: "Notes",
                table: "CorporateActions",
                newName: "notes");

            migrationBuilder.RenameColumn(
                name: "Id",
                table: "CorporateActions",
                newName: "id");

            migrationBuilder.RenameColumn(
                name: "SchemeCode",
                table: "CorporateActions",
                newName: "scheme_code");

            migrationBuilder.RenameColumn(
                name: "OldFaceValue",
                table: "CorporateActions",
                newName: "old_face_value");

            migrationBuilder.RenameColumn(
                name: "NewFaceValue",
                table: "CorporateActions",
                newName: "new_face_value");

            migrationBuilder.RenameColumn(
                name: "CreatedAtUtc",
                table: "CorporateActions",
                newName: "created_at_utc");

            migrationBuilder.RenameColumn(
                name: "AdjustmentFactor",
                table: "CorporateActions",
                newName: "adjustment_factor");

            migrationBuilder.RenameColumn(
                name: "ActionType",
                table: "CorporateActions",
                newName: "action_type");

            migrationBuilder.RenameColumn(
                name: "ActionDate",
                table: "CorporateActions",
                newName: "action_date");

            migrationBuilder.RenameIndex(
                name: "IX_CorporateActions_SchemeCode_ActionDate",
                table: "CorporateActions",
                newName: "ix_corporate_actions_scheme_code_action_date");

            migrationBuilder.RenameIndex(
                name: "IX_CorporateActions_SchemeCode",
                table: "CorporateActions",
                newName: "ix_corporate_actions_scheme_code");

            migrationBuilder.AddPrimaryKey(
                name: "pk_nav_sync_trackers",
                table: "NavSyncTrackers",
                column: "scheme_code");

            migrationBuilder.AddPrimaryKey(
                name: "pk_nav_histories",
                table: "NavHistories",
                column: "id");

            migrationBuilder.AddPrimaryKey(
                name: "pk_funds",
                table: "Funds",
                column: "scheme_code");

            migrationBuilder.AddPrimaryKey(
                name: "pk_corporate_actions",
                table: "CorporateActions",
                column: "id");

            migrationBuilder.AddForeignKey(
                name: "fk_corporate_actions_funds_scheme_code",
                table: "CorporateActions",
                column: "scheme_code",
                principalTable: "Funds",
                principalColumn: "scheme_code",
                onDelete: ReferentialAction.Cascade);

            migrationBuilder.AddForeignKey(
                name: "fk_nav_histories_funds_scheme_code",
                table: "NavHistories",
                column: "scheme_code",
                principalTable: "Funds",
                principalColumn: "scheme_code",
                onDelete: ReferentialAction.Cascade);

            migrationBuilder.AddForeignKey(
                name: "fk_nav_sync_trackers_funds_scheme_code",
                table: "NavSyncTrackers",
                column: "scheme_code",
                principalTable: "Funds",
                principalColumn: "scheme_code",
                onDelete: ReferentialAction.Cascade);
        }

        /// <inheritdoc />
        protected override void Down(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.DropForeignKey(
                name: "fk_corporate_actions_funds_scheme_code",
                table: "CorporateActions");

            migrationBuilder.DropForeignKey(
                name: "fk_nav_histories_funds_scheme_code",
                table: "NavHistories");

            migrationBuilder.DropForeignKey(
                name: "fk_nav_sync_trackers_funds_scheme_code",
                table: "NavSyncTrackers");

            migrationBuilder.DropPrimaryKey(
                name: "pk_nav_sync_trackers",
                table: "NavSyncTrackers");

            migrationBuilder.DropPrimaryKey(
                name: "pk_nav_histories",
                table: "NavHistories");

            migrationBuilder.DropPrimaryKey(
                name: "pk_funds",
                table: "Funds");

            migrationBuilder.DropPrimaryKey(
                name: "pk_corporate_actions",
                table: "CorporateActions");

            migrationBuilder.RenameColumn(
                name: "latest_nav_date_from_api",
                table: "NavSyncTrackers",
                newName: "LatestNavDateFromApi");

            migrationBuilder.RenameColumn(
                name: "last_check_had_new_data",
                table: "NavSyncTrackers",
                newName: "LastCheckHadNewData");

            migrationBuilder.RenameColumn(
                name: "last_api_check_utc",
                table: "NavSyncTrackers",
                newName: "LastApiCheckUtc");

            migrationBuilder.RenameColumn(
                name: "scheme_code",
                table: "NavSyncTrackers",
                newName: "SchemeCode");

            migrationBuilder.RenameColumn(
                name: "nav",
                table: "NavHistories",
                newName: "Nav");

            migrationBuilder.RenameColumn(
                name: "date",
                table: "NavHistories",
                newName: "Date");

            migrationBuilder.RenameColumn(
                name: "id",
                table: "NavHistories",
                newName: "Id");

            migrationBuilder.RenameColumn(
                name: "scheme_code",
                table: "NavHistories",
                newName: "SchemeCode");

            migrationBuilder.RenameIndex(
                name: "ix_nav_histories_scheme_code_date",
                table: "NavHistories",
                newName: "IX_NavHistories_SchemeCode_Date");

            migrationBuilder.RenameIndex(
                name: "ix_nav_histories_scheme_code",
                table: "NavHistories",
                newName: "IX_NavHistories_SchemeCode");

            migrationBuilder.RenameColumn(
                name: "updated_at",
                table: "Funds",
                newName: "UpdatedAt");

            migrationBuilder.RenameColumn(
                name: "scheme_type",
                table: "Funds",
                newName: "SchemeType");

            migrationBuilder.RenameColumn(
                name: "scheme_name",
                table: "Funds",
                newName: "SchemeName");

            migrationBuilder.RenameColumn(
                name: "scheme_category",
                table: "Funds",
                newName: "SchemeCategory");

            migrationBuilder.RenameColumn(
                name: "latest_nav_date",
                table: "Funds",
                newName: "LatestNavDate");

            migrationBuilder.RenameColumn(
                name: "latest_nav",
                table: "Funds",
                newName: "LatestNav");

            migrationBuilder.RenameColumn(
                name: "isin_growth",
                table: "Funds",
                newName: "IsinGrowth");

            migrationBuilder.RenameColumn(
                name: "isin_div_reinvestment",
                table: "Funds",
                newName: "IsinDivReinvestment");

            migrationBuilder.RenameColumn(
                name: "fund_house",
                table: "Funds",
                newName: "FundHouse");

            migrationBuilder.RenameColumn(
                name: "created_at",
                table: "Funds",
                newName: "CreatedAt");

            migrationBuilder.RenameColumn(
                name: "scheme_code",
                table: "Funds",
                newName: "SchemeCode");

            migrationBuilder.RenameColumn(
                name: "notes",
                table: "CorporateActions",
                newName: "Notes");

            migrationBuilder.RenameColumn(
                name: "id",
                table: "CorporateActions",
                newName: "Id");

            migrationBuilder.RenameColumn(
                name: "scheme_code",
                table: "CorporateActions",
                newName: "SchemeCode");

            migrationBuilder.RenameColumn(
                name: "old_face_value",
                table: "CorporateActions",
                newName: "OldFaceValue");

            migrationBuilder.RenameColumn(
                name: "new_face_value",
                table: "CorporateActions",
                newName: "NewFaceValue");

            migrationBuilder.RenameColumn(
                name: "created_at_utc",
                table: "CorporateActions",
                newName: "CreatedAtUtc");

            migrationBuilder.RenameColumn(
                name: "adjustment_factor",
                table: "CorporateActions",
                newName: "AdjustmentFactor");

            migrationBuilder.RenameColumn(
                name: "action_type",
                table: "CorporateActions",
                newName: "ActionType");

            migrationBuilder.RenameColumn(
                name: "action_date",
                table: "CorporateActions",
                newName: "ActionDate");

            migrationBuilder.RenameIndex(
                name: "ix_corporate_actions_scheme_code_action_date",
                table: "CorporateActions",
                newName: "IX_CorporateActions_SchemeCode_ActionDate");

            migrationBuilder.RenameIndex(
                name: "ix_corporate_actions_scheme_code",
                table: "CorporateActions",
                newName: "IX_CorporateActions_SchemeCode");

            migrationBuilder.AddPrimaryKey(
                name: "PK_NavSyncTrackers",
                table: "NavSyncTrackers",
                column: "SchemeCode");

            migrationBuilder.AddPrimaryKey(
                name: "PK_NavHistories",
                table: "NavHistories",
                column: "Id");

            migrationBuilder.AddPrimaryKey(
                name: "PK_Funds",
                table: "Funds",
                column: "SchemeCode");

            migrationBuilder.AddPrimaryKey(
                name: "PK_CorporateActions",
                table: "CorporateActions",
                column: "Id");

            migrationBuilder.AddForeignKey(
                name: "FK_CorporateActions_Funds_SchemeCode",
                table: "CorporateActions",
                column: "SchemeCode",
                principalTable: "Funds",
                principalColumn: "SchemeCode",
                onDelete: ReferentialAction.Cascade);

            migrationBuilder.AddForeignKey(
                name: "FK_NavHistories_Funds_SchemeCode",
                table: "NavHistories",
                column: "SchemeCode",
                principalTable: "Funds",
                principalColumn: "SchemeCode",
                onDelete: ReferentialAction.Cascade);

            migrationBuilder.AddForeignKey(
                name: "FK_NavSyncTrackers_Funds_SchemeCode",
                table: "NavSyncTrackers",
                column: "SchemeCode",
                principalTable: "Funds",
                principalColumn: "SchemeCode",
                onDelete: ReferentialAction.Cascade);
        }
    }
}
