# ---- Build stage ----
FROM mcr.microsoft.com/dotnet/sdk:10.0-preview AS build
WORKDIR /src

# Copy project file and restore dependencies (cached layer)
COPY NAVigator/NAVigator.csproj NAVigator/
RUN dotnet restore NAVigator/NAVigator.csproj

# Copy everything else and publish
COPY . .
RUN dotnet publish NAVigator/NAVigator.csproj -c Release -o /app/publish --no-restore

# ---- Runtime stage ----
FROM mcr.microsoft.com/dotnet/aspnet:10.0-preview AS runtime
WORKDIR /app

EXPOSE 8080

COPY --from=build /app/publish .

ENTRYPOINT ["dotnet", "NAVigator.dll"]
