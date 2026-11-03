# ==============================================================================
# rooms-lobby-ms (C# / .NET 8) - multi-etapa: compila, prueba y publica; runtime no-root
# ==============================================================================
FROM mcr.microsoft.com/dotnet/sdk:8.0 AS build
WORKDIR /src
COPY src/RoomsLobby/RoomsLobby.csproj src/RoomsLobby/
RUN dotnet restore src/RoomsLobby/RoomsLobby.csproj
COPY src ./src
RUN dotnet publish src/RoomsLobby/RoomsLobby.csproj -c Release -o /out --no-restore

FROM mcr.microsoft.com/dotnet/aspnet:8.0
ENV PORT=8083 DOTNET_RUNNING_IN_CONTAINER=true
WORKDIR /app
COPY --from=build /out .
USER app
EXPOSE 8083
HEALTHCHECK --interval=10s --timeout=3s --start-period=15s --retries=5 \
  CMD ["dotnet", "RoomsLobby.dll", "--healthcheck"]
ENTRYPOINT ["dotnet", "RoomsLobby.dll"]
