FROM mcr.microsoft.com/dotnet/sdk:10.0 AS build
WORKDIR /src
COPY NAVigator/NAVigator.csproj NAVigator/
RUN dotnet restore NAVigator/NAVigator.csproj
COPY . .
RUN dotnet publish NAVigator/NAVigator.csproj -c Release -o /app/publish --no-restore

FROM mcr.microsoft.com/dotnet/aspnet:10.0 AS runtime
WORKDIR /app
EXPOSE 8080
COPY --from=build /app/publish .
ENTRYPOINT ["dotnet", "NAVigator.dll"]